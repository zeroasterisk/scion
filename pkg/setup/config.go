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

	"github.com/GoogleCloudPlatform/scion/pkg/config"
)

// DeploymentTarget identifies where Scion will run agents.
type DeploymentTarget string

const (
	TargetWorkstation DeploymentTarget = "workstation"
	TargetGCEVM       DeploymentTarget = "gce-vm"
	TargetNAS         DeploymentTarget = "nas"
	TargetKubernetes  DeploymentTarget = "kubernetes"
)

// AuthMethod identifies how agents authenticate with LLM providers.
type AuthMethod string

const (
	AuthVertexAI       AuthMethod = "vertex-ai"
	AuthAPIKey         AuthMethod = "api-key"
	AuthEnvironment    AuthMethod = "environment"
	AuthNone           AuthMethod = "none"
)

// APIKeyProvider identifies which LLM provider the API key is for.
type APIKeyProvider string

const (
	ProviderClaude APIKeyProvider = "claude"
	ProviderGemini APIKeyProvider = "gemini"
)

// SetupAnswers collects all user responses from the wizard.
type SetupAnswers struct {
	// Deployment
	Target DeploymentTarget

	// Runtime (auto-detected or inferred from target)
	Runtime string

	// Authentication
	Auth          AuthMethod
	GCPProjectID  string
	GCPRegion     string
	SAKeyFilePath string
	APIKeyProvider APIKeyProvider

	// Hub
	HubHostname string
	HubPort     string

	// Image registry
	ImageRegistry string
}

// DefaultHubEndpoint returns the hub endpoint URL based on the deployment target and answers.
func (a *SetupAnswers) DefaultHubEndpoint() string {
	switch a.Target {
	case TargetWorkstation:
		return "http://localhost:8080"
	default:
		host := a.HubHostname
		if host == "" {
			host = "localhost"
		}
		port := a.HubPort
		if port == "" {
			port = "8080"
		}
		return fmt.Sprintf("http://%s:%s", host, port)
	}
}

// RuntimeName returns the runtime name for the settings file.
func (a *SetupAnswers) RuntimeName() string {
	if a.Runtime != "" {
		return a.Runtime
	}
	if a.Target == TargetKubernetes {
		return "kubernetes"
	}
	return "docker"
}

// ValidateTarget checks that a string is a valid deployment target.
func ValidateTarget(s string) (DeploymentTarget, error) {
	switch DeploymentTarget(s) {
	case TargetWorkstation, TargetGCEVM, TargetNAS, TargetKubernetes:
		return DeploymentTarget(s), nil
	default:
		return "", fmt.Errorf("invalid deployment target %q: must be one of workstation, gce-vm, nas, kubernetes", s)
	}
}

// ValidateAuth checks that a string is a valid authentication method.
func ValidateAuth(s string) (AuthMethod, error) {
	switch AuthMethod(s) {
	case AuthVertexAI, AuthAPIKey, AuthEnvironment, AuthNone:
		return AuthMethod(s), nil
	default:
		return "", fmt.Errorf("invalid auth method %q: must be one of vertex-ai, api-key, environment, none", s)
	}
}

// AuthSelectedType returns the auth_selectedType value for harness config.
func (a *SetupAnswers) AuthSelectedType() string {
	switch a.Auth {
	case AuthVertexAI:
		return "vertex-ai"
	case AuthAPIKey:
		return "api-key"
	case AuthEnvironment:
		return "" // let env vars handle it
	case AuthNone:
		return ""
	default:
		return ""
	}
}

// BuildSettings generates a config.Settings struct from the wizard answers.
func BuildSettings(answers *SetupAnswers) *config.Settings {
	hubEnabled := true
	runtimeName := answers.RuntimeName()

	settings := &config.Settings{
		ActiveProfile: "default",
		Hub: &config.HubClientConfig{
			Enabled:  &hubEnabled,
			Endpoint: answers.DefaultHubEndpoint(),
		},
		Runtimes: map[string]config.RuntimeConfig{
			runtimeName: {},
		},
		Profiles: map[string]config.ProfileConfig{
			"default": {
				Runtime: runtimeName,
			},
		},
		Harnesses: map[string]config.HarnessConfig{
			"claude": {
				AuthSelectedType: answers.AuthSelectedType(),
			},
			"gemini": {
				AuthSelectedType: answers.AuthSelectedType(),
			},
		},
	}

	// Add Kubernetes-specific config
	if answers.Target == TargetKubernetes {
		rtCfg := settings.Runtimes[runtimeName]
		rtCfg.Namespace = "scion"
		settings.Runtimes[runtimeName] = rtCfg
	}

	// Add Vertex AI environment variables
	if answers.Auth == AuthVertexAI {
		gcpEnv := map[string]string{}
		if answers.GCPProjectID != "" {
			gcpEnv["GOOGLE_CLOUD_PROJECT"] = answers.GCPProjectID
		}
		if answers.GCPRegion != "" {
			gcpEnv["GOOGLE_CLOUD_REGION"] = answers.GCPRegion
		}
		if len(gcpEnv) > 0 {
			claude := settings.Harnesses["claude"]
			claude.Env = gcpEnv
			settings.Harnesses["claude"] = claude

			// Copy env for gemini harness
			geminiEnv := make(map[string]string)
			for k, v := range gcpEnv {
				geminiEnv[k] = v
			}
			gemini := settings.Harnesses["gemini"]
			gemini.Env = geminiEnv
			settings.Harnesses["gemini"] = gemini
		}
	}

	return settings
}

// GenerateExampleConfig returns a commented YAML string suitable for
// a settings.yaml.example file.
func GenerateExampleConfig() string {
	return `# Scion Settings
# Generated by 'scion setup --manual'
# Rename this file to settings.yaml and edit to match your environment.

# Which profile to use by default
active_profile: default

# Hub configuration (coordinates agents across machines)
hub:
  enabled: true
  endpoint: http://localhost:8080
  # token: ""       # Set via 'scion hub auth login'
  # brokerId: ""    # Set automatically during broker registration

# Container runtimes
runtimes:
  docker:
    # host: ""      # Docker host (leave empty for local socket)
  # kubernetes:
  #   namespace: scion
  #   context: my-cluster

# Profiles bind a runtime to configuration overrides
profiles:
  default:
    runtime: docker
    # harness_overrides:
    #   claude:
    #     auth_selectedType: vertex-ai
    #   gemini:
    #     auth_selectedType: vertex-ai

# Harness configurations (per-LLM settings)
harnesses:
  claude:
    image: ""                     # Set via image_registry after building images
    auth_selectedType: api-key    # Options: api-key, vertex-ai, oauth-token, auth-file
    # env:
    #   GOOGLE_CLOUD_PROJECT: my-project
    #   GOOGLE_CLOUD_REGION: us-central1
  gemini:
    image: ""
    auth_selectedType: api-key
    # env:
    #   GOOGLE_CLOUD_PROJECT: my-project
    #   GOOGLE_CLOUD_REGION: us-central1
`
}
