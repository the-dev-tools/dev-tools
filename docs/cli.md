# DevTools CLI Guide

## Overview

The DevTools CLI (`devtoolscli`) is the command-line companion to the desktop application. It lets you execute exported workspaces, validate flows in continuous integration pipelines, and produce machine-readable reports. The CLI runs everything locally against an in-memory SQLite database, mirroring the behaviour of the server so you can rely on consistent results between manual testing and automated checks.

## Installation

The preferred installation method is the published release bundle. On macOS and Linux you can use the helper script:

```
curl -fsSL https://raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/cli/install.sh | bash
```

By default the script installs the binary to `/usr/local/bin`. Set `INSTALL_DIR` if you need another location. The CLI is cross-platform; Windows users can download the corresponding `.exe` from the releases page and place it somewhere on the `PATH`.

If you are hacking locally, run `pnpm install` and then `pnpm nx run cli:build` from the repo root. The compiled binary will appear under `apps/cli/dist`. Regardless of how you install, you can confirm your version with `devtoolscli version`.

## Running Flows from YAML

Export your workspace from the desktop app to produce a `.yamlflow.yaml` file. The CLI consumes that file with:

```
devtoolscli flow run path/to/workspace.yamlflow.yaml FlowName
```

If you omit the flow name the CLI reads the `run:` section and executes each entry in order, honouring `depends_on`. You can also point it at a simplified YAML using the same command; the importer handles both the legacy and the new structure transparently.

The CLI spins up a temporary SQLite database, imports every collection, endpoint, example, and flow, then runs the requested flow(s) through the same runner used by the server. Names, node types, assertions, scripts, and loop semantics all match what you see in the application.

## Environment Variable Overrides

Workspace environments travel with the exported data, but CI pipelines often need runtime overrides. Declare overrides in the `env:` block at the top level of your YAML:

```yaml
env:
  LOGIN_EMAIL: '#env:LOGIN_EMAIL'
  LOGIN_PASSWORD: '#env:LOGIN_PASSWORD'
```

`#env:NAME` instructs the CLI to read the process environment (`os.Getenv("NAME")`). If the variable is missing, the CLI falls back to whatever value is stored in the workspace. You can mix literal fallbacks and template references:

```yaml
env:
  API_KEY: 'plain-text-fallback'
  API_SECRET: '{{ secrets.MY_SECRET }}'
```

The importer normalises `${{ secrets.MY_SECRET }}` (and other `$` forms) to `#env:MY_SECRET`, so in GitHub Actions you can expose secrets with:

```yaml
steps:
  - run: devtoolscli flow run workspace.yamlflow.yaml FlowA
    env:
      LOGIN_EMAIL: ${{ secrets.LOGIN_EMAIL }}
      LOGIN_PASSWORD: ${{ secrets.LOGIN_PASSWORD }}
```

Inside the flow you continue to reference `{{ env.LOGIN_EMAIL }}` exactly as you would in the desktop app.

## Reports

By default the CLI prints a console report showing node order, duration, and status. You can request additional outputs with `--report format[:path]`. Supported formats are `console`, `json`, and `junit`, plus `frames:<url>` for load runs (see [Streaming frames](#streaming-frames)). Examples:

```
devtoolscli flow run workspace.yamlflow.yaml FlowA --report json:flow.json

devtoolscli flow run workspace.yamlflow.yaml FlowA --report console --report junit:flow.xml
```

You can specify the flag multiple times. When writing JSON or JUnit reports, the CLI appends flow results after each run and flushes them on exit. This is useful for CI systems that collect test artifacts.

## Load Testing

The same flows run as load tests. Either describe a constant-VU profile inline, or run a named entry of the file's `load:` block:

```
devtoolscli flow run shop.yaml Checkout --vus 20 --duration 60s
devtoolscli flow run shop.yaml --scenario checkout-ramp
devtoolscli flow run shop.yaml --scenario checkout-ramp --load-file stresseur.load.yaml
```

A `load:` entry picks one of four executors - `constant-vus`, `ramping-vus`, `constant-arrival-rate` (start iterations at a fixed rate whatever the response time, and count dropped iterations once `max_vus` are busy) and `ramping-arrival-rate` - and may add `think_time`, `thresholds` and `abort` rules. The schema is documented in the [YAML format README](../packages/server/pkg/translate/yamlflowsimplev2/README.md#load-scenarios). `--load-file` reads extra entries with the identical schema from a separate file.

A load run prints aggregate latency percentiles, throughput and error rate per request step. Only HTTP request steps are counted - not the `manual_start` node or other non-request nodes - and the JSON report carries the same data in its additive `load_report` field (`executor`, `requests`, `dropped_iterations`, `interrupted_iterations`, `thresholds_passed`, `aborted` and the per-threshold verdicts under `report.thresholds`).

| Flag                                  | Meaning                                                                                            |
| ------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `--scenario <name>`                   | Run that entry of the `load:` block (or of `--load-file`).                                         |
| `--vus`, `--duration`, `--iterations` | Inline constant-VU profile; mutually exclusive with `--scenario`.                                  |
| `--load-file <path>`                  | Add load scenarios from a separate file.                                                           |
| `--vus-scale <f>`                     | Multiply VU counts (and constant-vus iteration budgets) by `f`, e.g. `0.25` on each of 4 machines. |
| `--rate-scale <f>`                    | Multiply arrival rates by `f`.                                                                     |
| `--frame-interval <d>`                | How often metrics frames are cut (default `5s`): the streaming cadence and abort-rule interval.    |
| `--report frames:<url>`               | Stream every metrics frame to `<url>`, then the final report (see below).                          |

Exit codes: `0` for a completed run (even with failed requests), `99` when a threshold fails, `108` when an abort rule or the frames dead-man switch stopped the run, `1` when the run could not happen (unknown scenario, invalid profile, unreachable target).

### Streaming frames

`--report frames:<url>` POSTs a JSON envelope per metrics frame to `<url>` with `Authorization: Bearer $DEVTOOLS_FRAMES_TOKEN`:

```
{"kind":"frame","worker":"<DEVTOOLS_WORKER_ID or hostname>","scenario":"checkout-ramp","flow":"Checkout",
 "seq":0,"final":false,"active_vus":20,"dropped_iterations":0,"frame":{"intervalStart":"…","intervalMs":"5000","entries":[…]}}
```

`frame` is the `LoadMetricFrame` message (protobuf JSON) with one entry per request step and status class, each carrying its compressed HDR histogram, so frames from several machines merge without loss. After the run a final `{"kind":"report", …, "load_report": {…}}` envelope carries the same object as the JSON report. Delivery is retried with exponential backoff on network errors, 408, 429 and 5xx, and never slows the run: frames queue in the background and are dropped (and counted) if the queue fills. If the endpoint accepts nothing for 60 seconds the run ramps down and stops with exit code 108, so a load generator whose controller has gone away cannot keep hitting the target.

## Continuous Integration Tips

1. Check in your YAML flows and run them on every pull request. Combine `--report junit:…` with the CI system’s test report collector.
2. Use the `env:` block together with project secrets to avoid storing plaintext credentials in the repository.
3. If your flows depend on external APIs, run them against staging environments or mock servers to keep CI stable.
4. Consider adding `devtoolscli version` to your pipeline logs so you can diagnose regressions quickly.

### GitHub Actions

The bundled composite action downloads a released `devtoolscli` binary for
the runner's OS/arch, runs the flow, and publishes a job summary plus
JSON/JUnit reports as outputs — no repo checkout of DevTools itself or Nix/pnpm
toolchain needed. See [`actions/run-flows/README.md`](../actions/run-flows/README.md)
for the full inputs/outputs reference:

```yaml
jobs:
  flow-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: the-dev-tools/dev-tools/actions/run-flows@main
        with:
          file: flows.yamlflow.yaml
        env:
          LOGIN_EMAIL: ${{ secrets.LOGIN_EMAIL }}
          LOGIN_PASSWORD: ${{ secrets.LOGIN_PASSWORD }}
```

#### Manual alternative

Windows runners, air-gapped environments, or anything else the action doesn't
cover can install the CLI directly (see [Installation](#installation) above)
and call `flow run` themselves. Note the installed binary is named `devtools`,
not `devtoolscli`:

```yaml
jobs:
  flow-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -fsSL https://raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/cli/install.sh | bash
      - run: devtools flow run flows.yamlflow.yaml --report console --report junit:report.xml
        env:
          LOGIN_EMAIL: ${{ secrets.LOGIN_EMAIL }}
          LOGIN_PASSWORD: ${{ secrets.LOGIN_PASSWORD }}
```

## Unified Binary & CLI Mode

The binary is primarily a **server**. It can optionally act as a CLI when the `cli` build tag is included and the `DEVTOOLS_MODE` environment variable is set.

### Build Variants

| Variant     | Build Command                      | Includes CLI | Typical Use                                |
| ----------- | ---------------------------------- | ------------ | ------------------------------------------ |
| Server-only | `go build -o devtools .`           | No           | Desktop app backend, production deployment |
| Unified     | `go build -tags cli -o devtools .` | Yes          | All-in-one distribution, CI pipelines      |

The server-only build excludes CLI dependencies (Cobra, Viper, config management) and produces a smaller binary. The unified build links the CLI in via the `cli` build tag.

### Runtime Mode Selection

Set the `DEVTOOLS_MODE` environment variable to switch modes:

| Value     | Behaviour                                             |
| --------- | ----------------------------------------------------- |
| `server`  | Starts the HTTP server (Connect RPC)                  |
| `cli`     | Runs the CLI (Cobra commands: flow run, import, etc.) |
| _(unset)_ | Defaults to `server`                                  |

Any other value is rejected with an error.

```bash
# Run as server (default, DEVTOOLS_MODE unset)
./devtools

# Run as server (explicit)
DEVTOOLS_MODE=server ./devtools

# Run as CLI
DEVTOOLS_MODE=cli ./devtools flow run workspace.yamlflow.yaml FlowA
```

If `DEVTOOLS_MODE=cli` is set on a server-only build (compiled without `-tags cli`), the binary prints an error and exits.

### Desktop App Integration

The desktop Electron app spawns the binary as its backend. No `DEVTOOLS_MODE` is needed since server is the default:

```typescript
Command.env({
  DB_MODE: 'local',
  DB_NAME: 'state',
  DB_PATH: app.getPath('userData'),
  DB_ENCRYPTION_KEY: 'secret',
  HMAC_SECRET: 'secret',
});
```

The server requires the same environment variables as before (`DB_MODE`, `DB_NAME`, `DB_PATH`, `DB_ENCRYPTION_KEY`, `HMAC_SECRET`).

### Source Layout

- `apps/cli/main.go` — Entry point with mode switch and constants (`EnvDevToolsMode`, `ModeServer`, `ModeCLI`)
- `apps/cli/mode_cli.go` — Build-tagged file (`//go:build cli`) that wires the CLI commands
- `packages/server/cmd/serverrun/serverrun.go` — Extracted server startup logic, importable from any module in the workspace

## Debugging and Troubleshooting

- **Flow not found**: Ensure the `run` entry or flow name matches the exported data exactly (case-sensitive). Use `devtoolscli flow run workspace.yamlflow.yaml` without a name to list flows.
- **Missing environment variable**: When a `#env:NAME` override cannot resolve, the CLI logs the placeholder but continues with the stored value. Set the value explicitly in your CI environment or provide a literal fallback in the YAML.
- **Node failures**: The console report shows the first error encountered. Re-run with `LOG_LEVEL=DEBUG` to see detailed HTTP preparation and assertion logs.
- **External dependencies**: The CLI does not stub network calls. If you need deterministic runs, point your environment variables at mock servers or wrap the flows with conditionals.

## Getting Help

For bugs or feature requests file an issue on GitHub with the CLI version (`devtoolscli version`), the flow snippet that fails, and the console report. Pull requests are welcome; consult `docs/CONTRIBUTING.md` for coding standards and testing expectations. The CLI lives under `apps/cli/`; tests are in `apps/cli/cmd` and sample flows in `apps/cli/test/yamlflow/`.
