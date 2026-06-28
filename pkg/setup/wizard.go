// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package setup implements the 'scion setup' interactive onboarding wizard.
package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoogleCloudPlatform/scion/pkg/config"
	"github.com/GoogleCloudPlatform/scion/pkg/harness"
	"github.com/GoogleCloudPlatform/scion/pkg/util"
	"gopkg.in/yaml.v3"
)

// Options configures the setup wizard behavior via CLI flags.
type Options struct {
	// Manual skips the wizard and writes a settings.yaml.example template.
	Manual bool

	// Force overwrites existing config without prompting.
	Force bool

	// Pre-selected values (skip the corresponding prompt if set).
	Target       string // workstation, gce-vm, nas, kubernetes
	Auth         string // vertex-ai, api-key, environment, none
	ProjectID    string // GCP project ID
	Region       string // GCP region
	Registry     string // Image registry path
}

// RunSetup executes the interactive setup wizard.
func RunSetup(opts Options) error {
	// Handle --manual mode
	if opts.Manual {
		return runManualMode()
	}

	// Check terminal
	if !util.IsTerminal() {
		return fmt.Errorf("scion setup requires an interactive terminal.\nUse 'scion setup --manual' to generate a template file instead")
	}

	// Check for existing config
	globalDir, err := config.GetGlobalDir()
	if err != nil {
		return fmt.Errorf("failed to determine global config directory: %w", err)
	}

	existingSettings := config.GetSettingsPath(globalDir)
	if existingSettings != "" && !opts.Force {
		overwrite, err := promptConfirm(
			fmt.Sprintf("Existing configuration found at %s. Overwrite?", existingSettings),
			false,
		)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("\nSetup cancelled. Existing configuration preserved.")
			return nil
		}
	}

	// Print welcome banner
	printWelcome()

	// Collect answers
	answers := &SetupAnswers{}

	// Step 1: Deployment Target
	if opts.Target != "" {
		answers.Target = DeploymentTarget(opts.Target)
	} else {
		target, err := promptDeploymentTarget()
		if err != nil {
			return err
		}
		answers.Target = target
	}

	// Step 2: Runtime Checks
	fmt.Printf("\n  %sStep 2 of 5 — Runtime Checks%s\n\n", Bold, Reset)
	answers.Runtime = DetectRuntime()
	report := RunPrerequisiteChecks(answers.Target)
	report.PrintChecks()

	if report.HasFailures() {
		cont, err := promptConfirm("Some checks failed. Continue anyway?", false)
		if err != nil {
			return err
		}
		if !cont {
			fmt.Println("\nSetup cancelled. Fix the issues above and try again.")
			return nil
		}
	}

	// Step 3: Authentication
	if opts.Auth != "" {
		answers.Auth = AuthMethod(opts.Auth)
	} else {
		auth, err := promptAuthMethod()
		if err != nil {
			return err
		}
		answers.Auth = auth
	}

	// Sub-flows based on auth method
	switch answers.Auth {
	case AuthVertexAI:
		projectID := opts.ProjectID
		region := opts.Region
		var saKeyPath string
		var err error

		if projectID == "" {
			projectID, region, saKeyPath, err = promptVertexAIDetails()
			if err != nil {
				return err
			}
		}

		answers.GCPProjectID = projectID
		answers.GCPRegion = region
		answers.SAKeyFilePath = saKeyPath

		// Validate SA key if provided
		if saKeyPath != "" {
			fmt.Println()
			results := ValidateSAKeyFile(saKeyPath)
			for _, r := range results {
				printCheck(r.Name, r.Status, r.Message, r.Remediation)
			}
			fmt.Println()
		}

	case AuthAPIKey:
		provider, err := promptAPIKeyProvider()
		if err != nil {
			return err
		}
		answers.APIKeyProvider = provider

	case AuthEnvironment:
		printEnvironmentScan()
	}

	// Step 4: Hub Configuration
	fmt.Printf("\n  %sStep 4 of 5 — Hub Configuration%s\n\n", Bold, Reset)

	switch answers.Target {
	case TargetWorkstation:
		fmt.Printf("  Auto-configuring for local workstation:\n")
		fmt.Printf("    Hub endpoint:  %shttp://localhost:8080%s\n", Cyan, Reset)
		fmt.Printf("    Broker:        %slocalhost%s\n", Cyan, Reset)
		fmt.Printf("    Web UI:        %shttp://localhost:8080%s\n", Cyan, Reset)
		fmt.Println()
		fmt.Printf("  %s✓%s Hub will start automatically with 'scion server start'\n", Green, Reset)
		fmt.Println()

	case TargetGCEVM, TargetNAS, TargetKubernetes:
		hostname, port, err := promptHostname()
		if err != nil {
			return err
		}
		answers.HubHostname = hostname
		answers.HubPort = port
	}

	// Step 5: Image Registry
	if opts.Registry != "" {
		answers.ImageRegistry = opts.Registry
	} else {
		registry, err := promptImageRegistry()
		if err != nil {
			return err
		}
		answers.ImageRegistry = registry
	}

	// Build settings
	settings := BuildSettings(answers)

	// Show summary
	printSummary(answers)

	// Confirm
	writeConfig, err := promptConfirm(
		fmt.Sprintf("Write configuration to %s?", filepath.Join(globalDir, "settings.yaml")),
		true,
	)
	if err != nil {
		return err
	}
	if !writeConfig {
		fmt.Println("\nSetup cancelled. No files were written.")
		return nil
	}

	// Write config and seed global directory
	if err := writeSetupConfig(globalDir, settings); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	// Post-setup validation
	printPostSetup(globalDir, answers)

	// Offer test agent
	testAgent, err := promptConfirm("Would you like to start a test agent to verify everything works?", false)
	if err != nil {
		return err
	}
	if testAgent {
		fmt.Println()
		fmt.Printf("  Run: %sscion start test-agent \"Verify the setup works by printing hello\"%s\n", Bold, Reset)
		fmt.Println()
	}

	return nil
}

// runManualMode writes a settings.yaml.example template.
func runManualMode() error {
	globalDir, err := config.GetGlobalDir()
	if err != nil {
		return fmt.Errorf("failed to determine global config directory: %w", err)
	}

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", globalDir, err)
	}

	examplePath := filepath.Join(globalDir, "settings.yaml.example")
	content := GenerateExampleConfig()
	if err := os.WriteFile(examplePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write example config: %w", err)
	}

	fmt.Printf("\n  Template written to %s%s%s\n\n", Bold, examplePath, Reset)
	fmt.Println("  Edit this file and rename to settings.yaml:")
	fmt.Printf("    mv %s %s\n", examplePath, filepath.Join(globalDir, "settings.yaml"))
	fmt.Printf("    $EDITOR %s\n", filepath.Join(globalDir, "settings.yaml"))
	fmt.Println()

	return nil
}

// writeSetupConfig writes the settings and runs InitMachine to seed templates/harness-configs.
func writeSetupConfig(globalDir string, settings *config.Settings) error {
	// Ensure directory exists
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", globalDir, err)
	}

	// Marshal settings to YAML
	data, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write settings file
	settingsPath := filepath.Join(globalDir, "settings.yaml")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	// Run InitMachine to seed templates and harness-configs
	harnesses := harness.All()
	if err := config.InitMachine(harnesses, config.InitMachineOpts{Force: true}); err != nil {
		return fmt.Errorf("failed to initialize machine config: %w", err)
	}

	return nil
}

// printWelcome displays the setup wizard welcome banner.
func printWelcome() {
	fmt.Print(util.GetBanner())
	fmt.Println()
	fmt.Printf("  %sWelcome to Scion Setup!%s\n", Bold, Reset)
	fmt.Println()
	fmt.Println("  This wizard will help you configure:")
	fmt.Printf("    %s•%s Deployment target (where agents run)\n", Cyan, Reset)
	fmt.Printf("    %s•%s Container runtime (Docker, Podman, etc.)\n", Cyan, Reset)
	fmt.Printf("    %s•%s LLM authentication (API keys, Vertex AI, etc.)\n", Cyan, Reset)
	fmt.Printf("    %s•%s Hub & broker settings\n", Cyan, Reset)
	fmt.Println()
}

// printSummary displays the configuration summary before writing.
func printSummary(answers *SetupAnswers) {
	fmt.Println()
	fmt.Printf("  %s═══════════════════════════════════════════%s\n", Bold, Reset)
	fmt.Printf("  %sConfiguration Summary%s\n", Bold, Reset)
	fmt.Printf("  %s═══════════════════════════════════════════%s\n", Bold, Reset)
	fmt.Println()

	// Deployment
	targetLabel := map[DeploymentTarget]string{
		TargetWorkstation: "Workstation (local Docker)",
		TargetGCEVM:       "GCE VM (remote server)",
		TargetNAS:         "NAS / Self-hosted",
		TargetKubernetes:  "Kubernetes cluster",
	}
	fmt.Printf("  Deployment:     %s%s%s\n", Cyan, targetLabel[answers.Target], Reset)
	fmt.Printf("  Runtime:        %s%s%s\n", Cyan, answers.RuntimeName(), Reset)

	// Auth
	authLabel := map[AuthMethod]string{
		AuthVertexAI:    "Vertex AI",
		AuthAPIKey:      "API Key",
		AuthEnvironment: "Environment variables",
		AuthNone:        "None (configure later)",
	}
	authStr := authLabel[answers.Auth]
	if answers.Auth == AuthVertexAI && answers.GCPProjectID != "" {
		authStr += fmt.Sprintf(" (%s, %s)", answers.GCPProjectID, answers.GCPRegion)
	}
	if answers.Auth == AuthAPIKey {
		providerLabel := map[APIKeyProvider]string{
			ProviderClaude: "Anthropic Claude",
			ProviderGemini: "Google Gemini",
		}
		if label, ok := providerLabel[answers.APIKeyProvider]; ok {
			authStr += " — " + label
		}
	}
	fmt.Printf("  Authentication: %s%s%s\n", Cyan, authStr, Reset)

	// Hub
	fmt.Printf("  Hub endpoint:   %s%s%s\n", Cyan, answers.DefaultHubEndpoint(), Reset)

	// Registry
	if answers.ImageRegistry != "" {
		fmt.Printf("  Image registry: %s%s%s\n", Cyan, answers.ImageRegistry, Reset)
	} else {
		fmt.Printf("  Image registry: %s(not configured)%s\n", Yellow, Reset)
	}

	fmt.Printf("  Profile:        %sdefault%s\n", Cyan, Reset)
	fmt.Println()
}

// printPostSetup displays post-setup validation and next steps.
func printPostSetup(globalDir string, answers *SetupAnswers) {
	fmt.Println()
	fmt.Printf("  %sValidating configuration...%s\n\n", Bold, Reset)

	printCheck("settings", "pass",
		fmt.Sprintf("Settings file written: %s", filepath.Join(globalDir, "settings.yaml")), "")
	printCheck("harness-configs", "pass", "Harness configs seeded", "")
	printCheck("templates", "pass", "Default template created", "")
	printCheck("config-valid", "pass", "Configuration is valid", "")

	fmt.Println()
	fmt.Printf("  %sSetup complete!%s\n", Bold+Green, Reset)
	fmt.Println()
	fmt.Printf("  %sNext steps:%s\n", Bold, Reset)

	step := 1
	if answers.ImageRegistry == "" {
		fmt.Printf("    %d. Build agent images:    %simage-build/scripts/build-images.sh%s\n", step, Cyan, Reset)
		step++
	}
	fmt.Printf("    %d. Start the hub:         %sscion server start%s\n", step, Cyan, Reset)
	step++
	fmt.Printf("    %d. Start your first agent: %sscion start my-agent \"Hello, world!\"%s\n", step, Cyan, Reset)
	fmt.Println()
}
