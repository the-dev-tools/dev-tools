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

By default the CLI prints a console report showing node order, duration, and status. You can request additional outputs with `--report format[:path]`. Supported formats are `console`, `json`, and `junit`. Examples:

```
devtoolscli flow run workspace.yamlflow.yaml FlowA --report json:flow.json

devtoolscli flow run workspace.yamlflow.yaml FlowA --report console --report junit:flow.xml
```

You can specify the flag multiple times. When writing JSON or JUnit reports, the CLI appends flow results after each run and flushes them on exit. This is useful for CI systems that collect test artifacts.

## Continuous Integration Tips

1. Check in your YAML flows and run them on every pull request. Combine `--report junit:…` with the CI system’s test report collector.
2. Use the `env:` block together with project secrets to avoid storing plaintext credentials in the repository.
3. If your flows depend on external APIs, run them against staging environments or mock servers to keep CI stable.
4. Consider adding `devtoolscli version` to your pipeline logs so you can diagnose regressions quickly.

A minimal GitHub Actions job looks like:

```yaml
jobs:
  flow-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v3
        with:
          version: 9
      - run: pnpm install --frozen-lockfile
      - run: pnpm nx run cli:build
      - run: ./apps/cli/dist/devtoolscli flow run flows.yamlflow.yaml
        env:
          LOGIN_EMAIL: ${{ secrets.LOGIN_EMAIL }}
          LOGIN_PASSWORD: ${{ secrets.LOGIN_PASSWORD }}
```

## Debugging and Troubleshooting

- **Flow not found**: Ensure the `run` entry or flow name matches the exported data exactly (case-sensitive). Use `devtoolscli flow run workspace.yamlflow.yaml` without a name to list flows.
- **Missing environment variable**: When a `#env:NAME` override cannot resolve, the CLI logs the placeholder but continues with the stored value. Set the value explicitly in your CI environment or provide a literal fallback in the YAML.
- **Node failures**: The console report shows the first error encountered. Re-run with `LOG_LEVEL=DEBUG` to see detailed HTTP preparation and assertion logs.
- **External dependencies**: The CLI does not stub network calls. If you need deterministic runs, point your environment variables at mock servers or wrap the flows with conditionals.

## Getting Help

For bugs or feature requests file an issue on GitHub with the CLI version (`devtoolscli version`), the flow snippet that fails, and the console report. Pull requests are welcome; consult `docs/CONTRIBUTING.md` for coding standards and testing expectations. The CLI lives under `apps/cli/`; tests are in `apps/cli/cmd` and sample flows in `apps/cli/test/yamlflow/`.
