# Implementation Plan: Cloud Secret Manager Support (Issue #23)

## Problem

Users need to load secrets directly from cloud secret managers using a syntax
similar to the existing `{{#env:VAR_NAME}}` support. Currently there is no way
to reference secrets stored in GCP, AWS, or Azure from within request templates
or flow variables.

## Proposed Syntax

```
GCP:   {{#gcp:projects/p/secrets/oauth/versions/latest#client_secret}}
AWS:   {{#aws:secret-name#client_secret}}           (future)
Azure: {{#azure:vault/secret-name#client_secret}}   (future)
```

The optional `#fragment` suffix is a JSON field selector — if the secret value
is a JSON blob, the fragment extracts a specific key from it. Without a
fragment, the entire raw secret value is returned.

---

## Current Architecture

The interpolation engine lives in `packages/server/pkg/expression/interpolate.go`.
The `resolveVar()` method dispatches on prefix:

```
"#env:"  → resolveEnvVar()    — os.LookupEnv
"#file:" → resolveFileVar()   — os.ReadFile
default  → resolveExprVar()   — expr-lang evaluation
```

Key types: `UnifiedEnv` (environment), `InterpolationResult`, error types
(`EnvReferenceError`, `FileReferenceError`, `InterpolationError`).

The `UnifiedEnv` uses a builder pattern for optional capabilities:
`WithTracking(tracker)`, `WithFunc(name, fn)`.

---

## Architecture Decisions

### 1. Provider Interface + Injection (not direct imports)

The `expression` package should **not** import cloud SDKs directly. This would
force every consumer (including the CLI) to pull in heavy GCP/AWS dependencies
even when cloud secrets are not used.

Instead, define a `SecretResolver` interface injected into `UnifiedEnv` via a
`WithSecretResolver()` builder method — mirroring the `WithTracking()` pattern.

```go
// SecretResolver resolves cloud secret manager references.
type SecretResolver interface {
    ResolveSecret(ctx context.Context, provider, ref, fragment string) (string, error)
}
```

### 2. Package Structure

```
packages/server/pkg/secretresolver/
├── resolver.go          # SecretResolver interface
├── parse.go             # ParseSecretRef(ref) → (path, fragment)
├── fragment.go          # ExtractFragment(value, fragment) → string
├── multi.go             # MultiResolver — dispatches by provider name
└── gcpsecret/
    ├── gcp.go           # GCP Secret Manager implementation
    └── integration_gcp_test.go  # Integration test (build tag: gcp_integration)
```

The `expression` package imports only the interface from `secretresolver/`.
The concrete GCP implementation lives in `gcpsecret/` and is wired at startup.

### 3. Context Propagation

Currently `InterpolateCtx` accepts `context.Context` but the comment says
_"reserved for future use"_. Cloud secret resolution requires context for
network calls. This is the right time to thread context through the full chain:

- `InterpolateWithResultCtx(ctx, raw)` → `resolveVar(ctx, ...)` → `resolveSecretVar(ctx, ...)`
- Keep context-free `Interpolate()` / `InterpolateWithResult()` as convenience
  wrappers using `context.Background()` for backward compatibility.

### 4. Caching

Secrets are cached per-`GCPResolver` instance with a configurable TTL
(default: 5 minutes). Cache is keyed by `ref#fragment`. This avoids redundant
API calls when the same secret is referenced multiple times in a flow execution.

---

## File-by-File Changes

### New Files

| File                                                                   | Purpose                                                                   |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `packages/server/pkg/secretresolver/resolver.go`                       | `SecretResolver` interface                                                |
| `packages/server/pkg/secretresolver/parse.go`                          | `ParseSecretRef(ref) → (path, fragment)` using `strings.LastIndex("#")`   |
| `packages/server/pkg/secretresolver/fragment.go`                       | `ExtractFragment(value, fragment)` — JSON field extraction                |
| `packages/server/pkg/secretresolver/fragment_test.go`                  | Unit tests for fragment extraction                                        |
| `packages/server/pkg/secretresolver/parse_test.go`                     | Unit tests for reference parsing                                          |
| `packages/server/pkg/secretresolver/multi.go`                          | `MultiResolver` — provider dispatcher with `Register(provider, resolver)` |
| `packages/server/pkg/secretresolver/gcpsecret/gcp.go`                  | GCP implementation using `cloud.google.com/go/secretmanager/apiv1`        |
| `packages/server/pkg/secretresolver/gcpsecret/integration_gcp_test.go` | Integration test behind `gcp_integration` build tag                       |

### Modified Files

| File                                                 | Changes                                                                                                                 |
| ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `packages/server/pkg/expression/file_utils.go`       | Add `GCPRefPrefix`, `AWSRefPrefix`, `AzureRefPrefix` constants; `IsSecretReference()`, `ParseSecretReference()` helpers |
| `packages/server/pkg/expression/errors.go`           | Add `SecretReferenceError` type (mirrors `EnvReferenceError`)                                                           |
| `packages/server/pkg/expression/unified_env.go`      | Add `secretResolver` field to `UnifiedEnv`; `WithSecretResolver()` builder; update `Clone()`                            |
| `packages/server/pkg/expression/interpolate.go`      | Thread `context.Context` through `resolveVar()`; add `isSecretReference` case; add `resolveSecretVar()` method          |
| `packages/server/pkg/expression/unified_env_test.go` | Add tests with mock `SecretResolver`                                                                                    |
| `packages/server/go.mod`                             | Add `cloud.google.com/go/secretmanager` dependency                                                                      |

### Wiring Points (where resolver gets injected)

| Location              | Change                                                                  |
| --------------------- | ----------------------------------------------------------------------- |
| Flow builder (server) | Call `.WithSecretResolver(resolver)` when constructing `UnifiedEnv`     |
| CLI flow command      | Optionally create `GCPResolver` at startup, register in `MultiResolver` |

---

## Key Implementation Details

### Fragment Extraction

```
Input:  {{#gcp:projects/my-proj/secrets/oauth-creds/versions/latest#client_secret}}
         ^^^^  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^
         prefix              resource path                           fragment

1. Strip prefix "#gcp:" → "projects/my-proj/secrets/oauth-creds/versions/latest#client_secret"
2. ParseSecretRef (split on last '#') → path, fragment
3. GCP client fetches secret → '{"client_id":"abc","client_secret":"xyz"}'
4. ExtractFragment(raw, "client_secret") → "xyz"
```

If no fragment, the entire raw value is returned as-is.

### Secret Value Masking in Tracking

When the tracker is enabled, secret reads are recorded with `"***"` instead of
the actual value to prevent secret leakage in flow execution logs, the UI
variable inspector, or debug output.

```go
if e.tracker != nil {
    e.tracker.TrackRead(varRef, "***") // Never log actual secret
}
```

### Error Handling

New error type follows existing patterns:

```go
type SecretReferenceError struct {
    Provider string // "gcp", "aws", "azure"
    Ref      string // resource path
    Fragment string // optional JSON fragment key
    Cause    error
}
```

Error scenarios:

1. **No resolver configured** — user writes `{{#gcp:...}}` without wiring a resolver
2. **Empty path** — `{{#gcp:}}`
3. **GCP API error** — permission denied, secret not found, network timeout
4. **Fragment extraction failure** — value is not JSON, or key not found
5. **Unsupported provider** — `{{#aws:...}}` when only GCP is registered

### GCP Resolver Options

```go
resolver, err := gcpsecret.NewGCPResolver(ctx,
    gcpsecret.WithCacheTTL(5 * time.Minute),
)
```

Uses Application Default Credentials (ADC). No API keys accepted as parameters.

---

## Testing Strategy

### Unit Tests (no cloud access, no build tags)

**Mock resolver for expression tests:**

```go
type mockSecretResolver struct {
    secrets map[string]string
    err     error
}

func (m *mockSecretResolver) ResolveSecret(ctx context.Context, provider, ref, fragment string) (string, error) {
    if m.err != nil { return "", m.err }
    key := provider + ":" + ref + "#" + fragment
    val, ok := m.secrets[key]
    if !ok { return "", fmt.Errorf("secret not found") }
    return val, nil
}
```

**Test cases:**

- `TestInterpolate_GCPSecret_SimpleValue` — raw value resolution
- `TestInterpolate_GCPSecret_WithFragment` — JSON field extraction
- `TestInterpolate_GCPSecret_NoResolver` — clear error
- `TestInterpolate_GCPSecret_EmptyPath` — `ErrEmptyPath`
- `TestInterpolate_GCPSecret_MixedReferences` — `#env:` + `#gcp:` in same string
- `TestInterpolate_GCPSecret_TrackedAsMasked` — tracker records `"***"`
- `TestParseSecretRef_*` — path/fragment splitting
- `TestExtractFragment_*` — JSON extraction edge cases

### Integration Tests (behind build tag)

```go
//go:build gcp_integration

// Guard: RUN_GCP_INTEGRATION_TESTS=true
// Env: GCP_TEST_SECRET_NAME=projects/my-proj/secrets/test/versions/latest
```

Run: `RUN_GCP_INTEGRATION_TESTS=true go test -tags gcp_integration -v ./packages/server/pkg/secretresolver/gcpsecret/`

---

## Dependency Impact

**`packages/server/go.mod`**: Add `cloud.google.com/go/secretmanager`. The
server already has `cloud.google.com/go` and auth packages as transitive
dependencies from Vertex AI, so incremental cost is minimal.

**`apps/cli/go.mod`**: The CLI imports `packages/server` via `replace`
directive. The GCP Secret Manager dependency becomes transitive. However, Go's
dead code elimination ensures the GCP client code is only included in the
binary if the CLI actually imports `gcpsecret`. If binary size is a concern, a
`//go:build !nogcp` tag can be added later.

---

## Security Considerations

1. **Credential handling**: ADC only — no API keys or service account JSON as
   parameters. Users configure via `GOOGLE_APPLICATION_CREDENTIALS` or GCE metadata.
2. **Secret masking**: Tracker records `"***"`, not actual values.
3. **Error messages**: Never include secret values in error output.
4. **Cache scope**: Per-resolver instance, not global. Prevents cross-tenant leakage.
5. **Timeouts**: `context.Context` provides natural timeout control. Recommend
   10-second deadline per secret fetch.

---

## Phasing

### Phase 1 (This Issue): GCP + Core Infrastructure

- `secretresolver/` package with interface, parsing, fragment extraction
- `gcpsecret/` implementation
- `expression/` modifications (context threading, secret dispatch, error type)
- Unit tests with mock resolver
- Integration test behind build tag
- Wiring into flow builder and CLI
- Dependency update in `packages/server/go.mod`

### Phase 2 (Future): AWS Secrets Manager

- Create `secretresolver/awssecret/` package
- Register `"aws"` in `MultiResolver`
- Zero changes needed in `expression/` — `#aws:` prefix already handled

### Phase 3 (Future): Azure Key Vault

- Create `secretresolver/azuresecret/` package
- Register `"azure"` in `MultiResolver`
- Zero changes needed in `expression/` — `#azure:` prefix already handled

**Design-for-extensibility elements built into Phase 1:**

- `SecretResolver` interface is provider-agnostic
- `MultiResolver` dispatches by provider string
- `IsSecretReference()` already checks all three prefixes
- `ParseSecretReference()` already handles all three prefixes
- `SecretReferenceError` includes `Provider` field
