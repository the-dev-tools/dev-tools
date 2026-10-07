---
cli: patch
---

The CLI can be embedded in another Go program: `cmd.Root()` returns its root command, and each release publishes `devtools-go-src-<version>.tar.gz`, the Go source of the CLI and the modules it imports with the generated code included, to build against.
