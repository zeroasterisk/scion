# Design: `scion setup` — Interactive Onboarding Wizard

**Status:** Draft  
**Author:** Scion Team  
**Date:** 2026-06-28

## Overview

`scion setup` is a guided, interactive CLI wizard that walks first-time users through
configuring a Scion Hub installation. It replaces the current manual process of editing
`settings.yaml` by hand with a colorful, step-by-step experience inspired by `gh auth login`
and the Claude Code first-run flow.

## Goals

1. **Zero-to-running in under 2 minutes** — a new user should be able to start their first agent
2. **Validate as you go** — each step checks prerequisites before continuing
3. **Sensible defaults** — power users can skip the wizard entirely (`--manual`)
4. **Non-destructive** — existing config is detected and the user is warned before overwriting

## Non-Goals

- Replacing `scion init` (project-level init stays separate)
- Managing Hub server deployment (setup configures the *client* side)
- Kubernetes cluster provisioning

---

## User Journey

### Step 0: Pre-flight

Before the wizard starts:
- Check if `~/.scion/settings.yaml` already exists → warn and ask to overwrite or abort
- Check if running in a terminal (`util.IsTerminal()`) → abort with guidance if not

### Step 1: Welcome Banner

```
       .--(★)             █████████      █████████    █████      █████████     █████    ████
      /                  ███░░░░░███    ███░░░░░███  ░░███      ███░░░░░███   ░░█████   ███
     /                  ░███    ░░░    ███     ░░░    ░███     ███     ░███    ░██████  ███
    /                   ░░█████████   ░███            ░███    ░███     ░███    ░███░███ ███
---*-----(◈)             ░░░░░░░░███  ░███            ░███    ░███     ░███    ░███░░██████
    \                    ███    ░███  ░░███  ░░███    ░███    ░░███   ░███     ░███ ░░█████
     \                  ░░█████████    ░░█████████    █████    ░░█████████     █████  ░░████
      '--(▲)             ░░░░░░░░░      ░░░░░░░░░    ░░░░░      ░░░░░░░░░     ░░░░░    ░░░░

  Welcome to Scion Setup!

  This wizard will help you configure:
    • Deployment target (where agents run)
    • Container runtime (Docker, Podman, etc.)
    • LLM authentication (API keys, Vertex AI, etc.)
    • Hub & broker settings

  Press Enter to begin, or run 'scion setup --manual' for a blank template.
```

### Step 2: Deployment Target

```
  Step 1 of 5 — Deployment Target

  Where will Scion run your agents?

  > Workstation     Local Docker, single user (simplest)
    GCE VM          Remote server, accessible via domain
    NAS / Self-hosted  Docker on local network
    Kubernetes      Cluster-based deployment
```

Selection determines:
| Target | Runtime | Hub Mode | Network |
|--------|---------|----------|---------|
| Workstation | docker/podman (auto-detect) | localhost | localhost:8080 |
| GCE VM | docker | remote domain | user-provided hostname |
| NAS | docker | local network | user-provided hostname |
| Kubernetes | kubernetes | remote domain | user-provided hostname |

### Step 3: Runtime Checks

Based on the selected target, validate prerequisites. Uses animated spinners
during checks and the existing `printCheck` pattern for results:

```
  Step 2 of 5 — Runtime Checks

  ✓ git: git version 2.45.2
  ✓ docker: Docker version 27.1.1
  ✓ docker-daemon: Docker daemon is running
  ! tmux: tmux not found locally (required inside agent containers)

  3 checks passed, 1 warning
```

For **Kubernetes** target:
```
  ✓ kubectl: Client Version v1.30.2
  ✓ cluster: Connected to cluster "my-cluster"
  ✓ namespace: Namespace "scion" exists
```

If critical checks fail → show remediation and ask whether to continue anyway.

### Step 4: Authentication

```
  Step 3 of 5 — LLM Authentication

  How will agents authenticate with LLM providers?

  > Vertex AI         Google Cloud service account (recommended for GCP)
    API Key            Direct API key (Anthropic, Gemini, etc.)
    Environment Vars   Keys already in environment (advanced)
    None               Skip auth setup (configure later)
```

#### Vertex AI sub-flow:
```
  GCP Project ID: my-project-123
  Region [us-central1]: us-central1
  Service account key file: /path/to/sa-key.json

  ✓ Key file exists and is valid JSON
  ✓ Contains service_account type
  ✓ Project ID matches: my-project-123
```

#### API Key sub-flow:
```
  Which provider?
  > Claude (Anthropic)
    Gemini (Google)

  Anthropic API key: ********
  ✓ Key format looks valid (sk-ant-...)
```

#### Environment Vars sub-flow:
```
  Scanning environment for known keys...
  ✓ ANTHROPIC_API_KEY found
  ✗ GEMINI_API_KEY not found
  ✓ GOOGLE_APPLICATION_CREDENTIALS found

  Use detected credentials? [Y/n]
```

### Step 5: Hub & Broker Configuration

```
  Step 4 of 5 — Hub Configuration

  The Hub coordinates agents across machines.
```

For **Workstation**:
```
  Auto-configuring for local workstation:
    Hub endpoint:    http://localhost:8080
    Broker:          localhost
    Web UI:          http://localhost:8080

  ✓ Hub will start automatically with 'scion server start'
```

For **Remote** targets:
```
  Hostname or domain: agents.example.com
  Hub port [8080]: 8080
  Enable TLS? [y/N]: n

  Hub endpoint: http://agents.example.com:8080
```

### Step 6: Image Registry

```
  Step 5 of 5 — Image Registry

  Scion runs agents in containers. You need a registry for agent images.

  Image registry path (e.g., ghcr.io/myorg): ghcr.io/myteam

  ✓ Registry configured: ghcr.io/myteam
```

If skipped:
```
  ! Image registry not configured. Agents cannot start without it.
    Build images first: image-build/scripts/build-images.sh --registry <registry> --push
    Then run: scion config set --global image_registry <your-registry>
```

### Step 7: Review & Write Config

```
  ═══════════════════════════════════════════
  Configuration Summary
  ═══════════════════════════════════════════

  Deployment:     Workstation (local Docker)
  Runtime:        docker
  Authentication: Vertex AI (my-project-123, us-central1)
  Hub endpoint:   http://localhost:8080
  Image registry: ghcr.io/myteam
  Profile:        default

  Write configuration to ~/.scion/settings.yaml? [Y/n]
```

### Step 8: Post-Setup Validation

```
  Validating configuration...

  ✓ Settings file written: ~/.scion/settings.yaml
  ✓ Harness configs seeded
  ✓ Default template created
  ✓ Configuration is valid

  Setup complete! 🎉

  Next steps:
    1. Build agent images:  image-build/scripts/build-images.sh
    2. Start the hub:       scion server start
    3. Start your first agent: scion start my-agent "Hello, world!"
```

### Step 9: Optional Test Agent

```
  Would you like to start a test agent to verify everything works? [y/N]
```

If yes → runs `scion start test-agent "Verify the setup works by printing hello"`.

---

## Config Generation

### Workstation + Vertex AI

```yaml
active_profile: default
hub:
  enabled: true
  endpoint: http://localhost:8080
runtimes:
  docker:
    host: ""
profiles:
  default:
    runtime: docker
    harness_overrides:
      claude:
        auth_selectedType: vertex-ai
      gemini:
        auth_selectedType: vertex-ai
harnesses:
  claude:
    image: ""
    auth_selectedType: vertex-ai
    env:
      GOOGLE_CLOUD_PROJECT: "my-project-123"
      GOOGLE_CLOUD_REGION: "us-central1"
  gemini:
    image: ""
    auth_selectedType: vertex-ai
    env:
      GOOGLE_CLOUD_PROJECT: "my-project-123"
      GOOGLE_CLOUD_REGION: "us-central1"
```

### Workstation + API Key (Anthropic)

```yaml
active_profile: default
hub:
  enabled: true
  endpoint: http://localhost:8080
runtimes:
  docker:
    host: ""
profiles:
  default:
    runtime: docker
harnesses:
  claude:
    image: ""
    auth_selectedType: api-key
  gemini:
    image: ""
    auth_selectedType: api-key
```

### GCE VM + Vertex AI

```yaml
active_profile: default
hub:
  enabled: true
  endpoint: http://agents.example.com:8080
runtimes:
  docker:
    host: ""
profiles:
  default:
    runtime: docker
    harness_overrides:
      claude:
        auth_selectedType: vertex-ai
      gemini:
        auth_selectedType: vertex-ai
harnesses:
  claude:
    image: ""
    auth_selectedType: vertex-ai
    env:
      GOOGLE_CLOUD_PROJECT: "my-project-123"
      GOOGLE_CLOUD_REGION: "us-central1"
  gemini:
    image: ""
    auth_selectedType: vertex-ai
    env:
      GOOGLE_CLOUD_PROJECT: "my-project-123"
      GOOGLE_CLOUD_REGION: "us-central1"
```

### Kubernetes

```yaml
active_profile: default
hub:
  enabled: true
  endpoint: http://k8s-host.example.com:8080
runtimes:
  kubernetes:
    namespace: scion
    context: my-cluster
profiles:
  default:
    runtime: kubernetes
harnesses:
  claude:
    image: ""
    auth_selectedType: vertex-ai
  gemini:
    image: ""
    auth_selectedType: vertex-ai
```

---

## Advanced Users

### `--manual` flag

```
scion setup --manual
```

Creates `~/.scion/settings.yaml.example` with a fully commented template and prints:

```
  Template written to ~/.scion/settings.yaml.example

  Edit this file and rename to settings.yaml:
    mv ~/.scion/settings.yaml.example ~/.scion/settings.yaml
    $EDITOR ~/.scion/settings.yaml
```

### Skipping steps

Each step in the wizard can be skipped via flags:

| Flag | Effect |
|------|--------|
| `--manual` | Skip wizard entirely, write example template |
| `--target workstation` | Pre-select deployment target |
| `--auth vertex-ai` | Pre-select auth method |
| `--project-id ID` | Pre-set GCP project ID |
| `--region REGION` | Pre-set GCP region |
| `--registry PATH` | Pre-set image registry |
| `--force` | Overwrite existing config without prompting |

---

## Implementation Structure

```
cmd/
  setup.go              # Cobra command definition (thin)

pkg/setup/
  wizard.go             # Main wizard orchestration (RunSetup)
  prompts.go            # huh-based interactive prompt helpers
  checks.go             # Runtime/prerequisite validation checks
  config.go             # Settings generation from wizard answers
```

### Package Boundaries

- `cmd/setup.go`: Registers the cobra command, parses flags, calls `setup.RunSetup()`
- `pkg/setup/wizard.go`: Orchestrates the full wizard flow, calls prompts and checks
- `pkg/setup/prompts.go`: Reusable prompt functions using `charmbracelet/huh`
- `pkg/setup/checks.go`: System validation (Docker, git, connectivity, SA key validation)
- `pkg/setup/config.go`: Builds a `config.Settings` struct from wizard answers

### Dependencies

- `charmbracelet/huh` — styled interactive prompts (select menus, text input, confirm)
- Existing: `pkg/config` (Settings, SaveSettings, InitMachine)
- Existing: `pkg/util` (IsTerminal, color constants, GetBanner)
- Existing: `pkg/harness` (All() for harness seeding)
