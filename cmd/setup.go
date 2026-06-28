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

package cmd

import (
	"github.com/GoogleCloudPlatform/scion/pkg/setup"
	"github.com/spf13/cobra"
)

var setupOpts setup.Options

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard for first-time Scion configuration",
	Long: `Run an interactive wizard to configure Scion for first-time use.

The wizard guides you through:
  • Choosing a deployment target (workstation, VM, NAS, Kubernetes)
  • Validating system prerequisites (Docker, git, etc.)
  • Configuring LLM authentication (Vertex AI, API keys, etc.)
  • Setting up Hub and broker connectivity
  • Configuring the container image registry

The generated configuration is written to ~/.scion/settings.yaml.

Use --manual to skip the wizard and generate a commented template file instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return setup.RunSetup(setupOpts)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().BoolVar(&setupOpts.Manual, "manual", false,
		"Skip the wizard and write a settings.yaml.example template")
	setupCmd.Flags().BoolVar(&setupOpts.Force, "force", false,
		"Overwrite existing configuration without prompting")
	setupCmd.Flags().StringVar(&setupOpts.Target, "target", "",
		"Pre-select deployment target (workstation, gce-vm, nas, kubernetes)")
	setupCmd.Flags().StringVar(&setupOpts.Auth, "auth", "",
		"Pre-select authentication method (vertex-ai, api-key, environment, none)")
	setupCmd.Flags().StringVar(&setupOpts.ProjectID, "project-id", "",
		"Pre-set GCP project ID (for vertex-ai auth)")
	setupCmd.Flags().StringVar(&setupOpts.Region, "region", "",
		"Pre-set GCP region (for vertex-ai auth)")
	setupCmd.Flags().StringVar(&setupOpts.Registry, "registry", "",
		"Pre-set container image registry path")
}
