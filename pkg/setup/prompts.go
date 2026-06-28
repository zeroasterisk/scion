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

package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/scion/pkg/util"
	"github.com/charmbracelet/huh"
)

// Re-export color constants for convenience within this package.
var (
	Green = util.Green
	Red   = util.Red
	Yellow = util.Yellow
	Gray  = util.Gray
	Bold  = util.Bold
	Reset = util.Reset
	Cyan  = util.Cyan
)

// promptDeploymentTarget asks the user to select a deployment target.
func promptDeploymentTarget() (DeploymentTarget, error) {
	var target string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where will Scion run your agents?").
				Description("Choose the deployment target that best matches your setup.").
				Options(
					huh.NewOption("Workstation — Local Docker, single user (simplest)", string(TargetWorkstation)),
					huh.NewOption("GCE VM — Remote server, accessible via domain", string(TargetGCEVM)),
					huh.NewOption("NAS / Self-hosted — Docker on local network", string(TargetNAS)),
					huh.NewOption("Kubernetes — Cluster-based deployment", string(TargetKubernetes)),
				).
				Value(&target),
		).Title("Step 1 of 5 — Deployment Target"),
	)

	if err := form.Run(); err != nil {
		return "", err
	}
	return DeploymentTarget(target), nil
}

// promptAuthMethod asks the user to select an authentication method.
func promptAuthMethod() (AuthMethod, error) {
	var auth string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How will agents authenticate with LLM providers?").
				Options(
					huh.NewOption("Vertex AI — Google Cloud service account (recommended for GCP)", string(AuthVertexAI)),
					huh.NewOption("API Key — Direct API key (Anthropic, Gemini, etc.)", string(AuthAPIKey)),
					huh.NewOption("Environment Vars — Keys already in environment (advanced)", string(AuthEnvironment)),
					huh.NewOption("None — Skip auth setup (configure later)", string(AuthNone)),
				).
				Value(&auth),
		).Title("Step 3 of 5 — LLM Authentication"),
	)

	if err := form.Run(); err != nil {
		return "", err
	}
	return AuthMethod(auth), nil
}

// promptVertexAIDetails asks for GCP project ID, region, and optionally SA key path.
func promptVertexAIDetails() (projectID, region, saKeyPath string, err error) {
	region = "us-central1" // default

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("GCP Project ID").
				Description("Your Google Cloud project ID").
				Placeholder("my-project-123").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("project ID is required")
					}
					return nil
				}).
				Value(&projectID),
			huh.NewInput().
				Title("Region").
				Description("Google Cloud region for Vertex AI").
				Placeholder("us-central1").
				Value(&region),
			huh.NewInput().
				Title("Service Account Key File (optional)").
				Description("Path to a JSON key file. Leave blank to use Application Default Credentials.").
				Placeholder("/path/to/sa-key.json").
				Value(&saKeyPath),
		).Title("Vertex AI Configuration"),
	)

	err = form.Run()
	if err != nil {
		return "", "", "", err
	}

	// Apply defaults
	projectID = strings.TrimSpace(projectID)
	region = strings.TrimSpace(region)
	if region == "" {
		region = "us-central1"
	}
	saKeyPath = strings.TrimSpace(saKeyPath)

	return projectID, region, saKeyPath, nil
}

// promptAPIKeyProvider asks which LLM provider the API key is for.
func promptAPIKeyProvider() (APIKeyProvider, error) {
	var provider string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which LLM provider?").
				Options(
					huh.NewOption("Claude (Anthropic)", string(ProviderClaude)),
					huh.NewOption("Gemini (Google)", string(ProviderGemini)),
				).
				Value(&provider),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}
	return APIKeyProvider(provider), nil
}

// promptHostname asks for a hostname/domain for remote deployments.
func promptHostname() (hostname, port string, err error) {
	port = "8080" // default

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Hostname or domain").
				Description("The address where the Hub will be accessible").
				Placeholder("agents.example.com").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("hostname is required for remote deployments")
					}
					return nil
				}).
				Value(&hostname),
			huh.NewInput().
				Title("Hub port").
				Placeholder("8080").
				Value(&port),
		).Title("Hub Network Configuration"),
	)

	err = form.Run()
	if err != nil {
		return "", "", err
	}

	hostname = strings.TrimSpace(hostname)
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8080"
	}

	return hostname, port, nil
}

// promptImageRegistry asks for the container image registry path.
func promptImageRegistry() (string, error) {
	var registry string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Image registry path").
				Description("Container registry for agent images (leave blank to configure later)").
				Placeholder("ghcr.io/myorg").
				Value(&registry),
		).Title("Step 5 of 5 — Image Registry"),
	)

	if err := form.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(registry), nil
}

// promptConfirm asks a yes/no question.
func promptConfirm(title string, defaultVal bool) (bool, error) {
	var confirmed bool = defaultVal

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// printEnvironmentScan scans the local environment for known LLM keys and
// prints the results.
func printEnvironmentScan() {
	keys := []struct {
		envVar string
		label  string
	}{
		{"ANTHROPIC_API_KEY", "Anthropic API Key"},
		{"GEMINI_API_KEY", "Gemini API Key"},
		{"GOOGLE_API_KEY", "Google API Key"},
		{"GOOGLE_APPLICATION_CREDENTIALS", "Google Application Credentials"},
		{"GOOGLE_CLOUD_PROJECT", "Google Cloud Project"},
		{"GOOGLE_CLOUD_REGION", "Google Cloud Region"},
	}

	fmt.Println()
	fmt.Println("  Scanning environment for known credentials...")
	fmt.Println()

	found := 0
	for _, k := range keys {
		val := os.Getenv(k.envVar)
		if val != "" {
			fmt.Printf("  %s✓%s %s (%s)\n", Green, Reset, k.label, k.envVar)
			found++
		} else {
			fmt.Printf("  %s·%s %s (%s) — not set\n", Gray, Reset, k.label, k.envVar)
		}
	}
	fmt.Println()
	if found > 0 {
		fmt.Printf("  %d credential(s) found in environment.\n", found)
	} else {
		fmt.Println("  No credentials found in environment.")
	}
	fmt.Println()
}
