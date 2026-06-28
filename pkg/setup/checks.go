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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/util"
)

// CheckResult holds the outcome of a single prerequisite check.
type CheckResult struct {
	Name        string
	Status      string // "pass", "warn", "fail"
	Message     string
	Remediation string
}

// CheckReport holds all check results for a target.
type CheckReport struct {
	Checks  []CheckResult
	Passes  int
	Warns   int
	Fails   int
}

// RunPrerequisiteChecks validates system prerequisites for the given deployment target.
func RunPrerequisiteChecks(target DeploymentTarget) *CheckReport {
	report := &CheckReport{}

	// Universal checks
	report.addCheck(checkGit())
	report.addCheck(checkTmux())

	// Target-specific checks
	switch target {
	case TargetWorkstation, TargetGCEVM, TargetNAS:
		report.addCheck(checkDockerOrPodman())
	case TargetKubernetes:
		report.addCheck(checkKubectl())
	}

	return report
}

func (r *CheckReport) addCheck(c CheckResult) {
	r.Checks = append(r.Checks, c)
	switch c.Status {
	case "pass":
		r.Passes++
	case "warn":
		r.Warns++
	case "fail":
		r.Fails++
	}
}

// HasFailures returns true if any check failed.
func (r *CheckReport) HasFailures() bool {
	return r.Fails > 0
}

// PrintChecks renders the check results to stdout using the doctor.go style.
func (r *CheckReport) PrintChecks() {
	for _, c := range r.Checks {
		printCheck(c.Name, c.Status, c.Message, c.Remediation)
	}
	fmt.Println()
	if r.Fails > 0 {
		fmt.Printf("  %s%d passed, %d warnings, %d failures%s\n",
			util.Red, r.Passes, r.Warns, r.Fails, util.Reset)
	} else if r.Warns > 0 {
		fmt.Printf("  %s%d passed, %d warnings%s\n",
			util.Yellow, r.Passes, r.Warns, util.Reset)
	} else {
		fmt.Printf("  %s%d passed%s\n",
			util.Green, r.Passes, util.Reset)
	}
}

func printCheck(name, status, message, remediation string) {
	var icon string
	switch status {
	case "pass":
		icon = fmt.Sprintf("%s✓%s", util.Green, util.Reset)
	case "warn":
		icon = fmt.Sprintf("%s!%s", util.Yellow, util.Reset)
	case "fail":
		icon = fmt.Sprintf("%s✗%s", util.Red, util.Reset)
	}
	fmt.Printf("  %s %s: %s\n", icon, name, message)
	if remediation != "" && status != "pass" {
		fmt.Printf("    → %s\n", remediation)
	}
}

func checkGit() CheckResult {
	path, err := exec.LookPath("git")
	if err != nil {
		return CheckResult{
			Name:        "git",
			Status:      "fail",
			Message:     "git not found in PATH",
			Remediation: "Install git: https://git-scm.com/downloads",
		}
	}
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return CheckResult{
			Name:    "git",
			Status:  "warn",
			Message: fmt.Sprintf("git found at %s but version check failed", path),
		}
	}
	return CheckResult{
		Name:    "git",
		Status:  "pass",
		Message: strings.TrimSpace(string(out)),
	}
}

func checkTmux() CheckResult {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return CheckResult{
			Name:        "tmux",
			Status:      "warn",
			Message:     "tmux not found locally (required inside agent containers)",
			Remediation: "Install tmux if you plan to attach to agents",
		}
	}
	out, err := exec.Command("tmux", "-V").Output()
	if err != nil {
		return CheckResult{
			Name:    "tmux",
			Status:  "pass",
			Message: "tmux found",
		}
	}
	return CheckResult{
		Name:    "tmux",
		Status:  "pass",
		Message: strings.TrimSpace(string(out)),
	}
}

func checkDockerOrPodman() CheckResult {
	// Try podman first, then docker
	for _, name := range []string{"podman", "docker"} {
		_, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(name, "--version").Output()
		if err != nil {
			continue
		}

		// Check daemon connectivity (with timeout to avoid hanging)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, daemonErr := exec.CommandContext(ctx, name, "info").Output()
		cancel()
		if daemonErr != nil {
			return CheckResult{
				Name:        name,
				Status:      "fail",
				Message:     fmt.Sprintf("%s found (%s) but daemon is not running", name, strings.TrimSpace(string(out))),
				Remediation: fmt.Sprintf("Start the %s daemon and try again", name),
			}
		}

		return CheckResult{
			Name:    name,
			Status:  "pass",
			Message: strings.TrimSpace(string(out)),
		}
	}

	return CheckResult{
		Name:        "container-runtime",
		Status:      "fail",
		Message:     "No container runtime found (docker or podman)",
		Remediation: "Install Docker: https://docs.docker.com/get-docker/ or Podman: https://podman.io/getting-started/installation",
	}
}

func checkKubectl() CheckResult {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return CheckResult{
			Name:        "kubectl",
			Status:      "fail",
			Message:     "kubectl not found in PATH",
			Remediation: "Install kubectl: https://kubernetes.io/docs/tasks/tools/",
		}
	}
	out, err := exec.Command("kubectl", "version", "--client", "--output=json").Output()
	if err != nil {
		return CheckResult{
			Name:    "kubectl",
			Status:  "warn",
			Message: "kubectl found but version check failed",
		}
	}
	return CheckResult{
		Name:    "kubectl",
		Status:  "pass",
		Message: fmt.Sprintf("kubectl %s", strings.TrimSpace(string(out))),
	}
}

// ValidateSAKeyFile checks if a service account key file exists and has valid structure.
func ValidateSAKeyFile(path string) []CheckResult {
	var results []CheckResult

	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "sa-key-file",
			Status:      "fail",
			Message:     fmt.Sprintf("File not found: %s", path),
			Remediation: "Check the file path and try again",
		})
		return results
	}
	if info.IsDir() {
		results = append(results, CheckResult{
			Name:        "sa-key-file",
			Status:      "fail",
			Message:     fmt.Sprintf("Path is a directory, not a file: %s", path),
			Remediation: "Provide the path to the JSON key file, not its parent directory",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:    "sa-key-file",
		Status:  "pass",
		Message: fmt.Sprintf("File exists: %s", path),
	})

	// Check valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "sa-key-json",
			Status:      "fail",
			Message:     fmt.Sprintf("Cannot read file: %v", err),
			Remediation: "Check file permissions",
		})
		return results
	}

	var keyData map[string]interface{}
	if err := json.Unmarshal(data, &keyData); err != nil {
		results = append(results, CheckResult{
			Name:        "sa-key-json",
			Status:      "fail",
			Message:     "File is not valid JSON",
			Remediation: "Download a fresh service account key from the GCP console",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:    "sa-key-json",
		Status:  "pass",
		Message: "Valid JSON structure",
	})

	// Check it's a service account key
	keyType, _ := keyData["type"].(string)
	if keyType != "service_account" {
		results = append(results, CheckResult{
			Name:        "sa-key-type",
			Status:      "fail",
			Message:     fmt.Sprintf("Key type is %q, expected \"service_account\"", keyType),
			Remediation: "Use a service account key, not an OAuth or authorized user key",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:    "sa-key-type",
		Status:  "pass",
		Message: "Contains service_account type",
	})

	// Check project ID if present
	projectID, _ := keyData["project_id"].(string)
	if projectID != "" {
		results = append(results, CheckResult{
			Name:    "sa-key-project",
			Status:  "pass",
			Message: fmt.Sprintf("Project ID: %s", projectID),
		})
	}

	return results
}

// DetectRuntime returns the name of the detected container runtime.
// Returns an empty string if no runtime is found.
func DetectRuntime() string {
	for _, name := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}
