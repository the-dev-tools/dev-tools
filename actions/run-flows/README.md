# run-flows

Composite GitHub Action that runs a DevTools `.yamlflow.yaml` file with the
released `devtoolscli` binary, publishes a job summary, and produces JSON /
JUnit reports as step outputs.

It downloads the `devtoolscli` release binary for the runner's OS/arch itself
— the consuming workflow only needs to check out its own repo (the one
containing the `.yamlflow.yaml` file), not this one:

```yaml
- uses: the-dev-tools/dev-tools/actions/run-flows@main
  with:
    file: flows/smoke.yamlflow.yaml
```

Pin `@main` to a commit SHA (or a `cli@<version>` tag, which is a normal git
tag on this monorepo) if you want the action's own behavior to stay fixed
independently of the `version` input.

## Supported runners

Linux and macOS only (`ubuntu-*`, `macos-*` runners), `x64` and `arm64`.
Windows is out of scope for this action: `devtoolscli` does publish Windows
release assets (see `.github/workflows/release-go.yaml`), but this action
does not resolve or invoke them, and every step is `shell: bash`. Windows CI
should use the manual install steps in [`docs/cli.md`](../../docs/cli.md)
instead.

## Inputs

| Name            | Required | Default             | Description                                                                                                 |
| --------------- | -------- | ------------------- | ----------------------------------------------------------------------------------------------------------- |
| `file`          | yes      | —                   | Path to the `.yamlflow.yaml` file to run.                                                                   |
| `flow`          | no       | _(unset)_           | Single flow name to run. Defaults to the file's top-level `run:` block.                                     |
| `version`       | no       | `latest`            | `devtoolscli` release to install: `latest`, a release tag (`cli@1.0.3`), or a bare version (`1.0.3`).       |
| `report-dir`    | no       | `.devtools-reports` | Directory to write the JSON and JUnit reports into.                                                         |
| `fail-on-error` | no       | `true`              | Fail this step if any flow fails. Set to `'false'` to always exit 0 and check the `success` output instead. |

## Outputs

| Name           | Description                                                       |
| -------------- | ----------------------------------------------------------------- |
| `json-report`  | Path to the JSON report (empty string if none was produced).      |
| `junit-report` | Path to the JUnit XML report (empty string if none was produced). |
| `success`      | `'true'` if every flow in the run succeeded, `'false'` otherwise. |

A job summary table (flow name, ✅/❌ status, duration) is always published to
the job's summary, even when `fail-on-error: 'false'` or the run fails —
useful for `if: always()` follow-up steps.

## Examples

### PR check

```yaml
name: API flows
on:
  pull_request:
jobs:
  flows:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: the-dev-tools/dev-tools/actions/run-flows@main
        with:
          file: flows/smoke.yamlflow.yaml
```

### Nightly cron against staging

```yaml
name: Nightly flow check
on:
  schedule:
    - cron: '0 6 * * *'
jobs:
  flows:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: the-dev-tools/dev-tools/actions/run-flows@main
        id: flows
        with:
          file: flows/nightly.yamlflow.yaml
          version: cli@1.0.3
          fail-on-error: 'false'
        env:
          LOGIN_EMAIL: ${{ secrets.LOGIN_EMAIL }}
          LOGIN_PASSWORD: ${{ secrets.LOGIN_PASSWORD }}
      - name: Notify on failure
        if: steps.flows.outputs.success != 'true'
        run: echo "Nightly flows failed — see the job summary for details" # replace with a real notification step
```

`env:` overrides for the YAML file's `#env:NAME` placeholders work the same
way as in the manual CLI usage documented in
[`docs/cli.md`](../../docs/cli.md#environment-variable-overrides).

## How it works

1. Resolves `version` to a release tag (`cli@<version>`) and downloads the
   matching `devtools-cli-<version>-<os>-<arch>` asset from this repo's
   GitHub Releases into `$RUNNER_TEMP/devtools/bin`. `latest` resolves to the
   highest `cli@*` tag via `git ls-remote` (the repo also cuts `desktop@`/
   `web@`/etc. releases, so a plain "latest release" API lookup would not be
   specific enough).
2. Runs `devtoolscli flow run <file> [flow] --report console --report json:<report-dir>/report.json --report junit:<report-dir>/junit.xml`.
3. Publishes a job summary table from the JSON report, whether or not the run
   succeeded.
4. Sets the `json-report` / `junit-report` / `success` outputs, then fails
   the step if `fail-on-error` is `'true'` (the default) and any flow failed.

See `apps/cli/internal/reporter/reporter.go` for the JSON report schema and
[`docs/cli.md`](../../docs/cli.md) for the YAML flow format and the
`{{ ... }}` interpolation syntax used in `testdata/smoke.yamlflow.yaml`.
