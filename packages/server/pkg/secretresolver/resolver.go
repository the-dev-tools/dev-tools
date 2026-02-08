// Package secretresolver defines the interface and utilities for resolving
// cloud secret manager references (e.g., GCP Secret Manager, AWS Secrets Manager).
package secretresolver

import "context"

// SecretResolver resolves cloud secret manager references.
// The provider identifies the cloud platform ("gcp", "aws", "azure").
// The ref is the provider-specific resource path.
// The fragment is an optional JSON field name to extract from the secret value.
type SecretResolver interface {
	ResolveSecret(ctx context.Context, provider, ref, fragment string) (string, error)
}
