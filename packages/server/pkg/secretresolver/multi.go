package secretresolver

import (
	"context"
	"fmt"
	"strings"
)

// MultiResolver dispatches to provider-specific resolvers.
type MultiResolver struct {
	providers map[string]SecretResolver
}

// NewMultiResolver creates a resolver that dispatches by provider name.
func NewMultiResolver() *MultiResolver {
	return &MultiResolver{providers: make(map[string]SecretResolver)}
}

// Register adds a provider-specific resolver.
func (m *MultiResolver) Register(provider string, resolver SecretResolver) {
	m.providers[provider] = resolver
}

// ResolveSecret dispatches to the registered provider.
func (m *MultiResolver) ResolveSecret(ctx context.Context, provider, ref, fragment string) (string, error) {
	r, ok := m.providers[provider]
	if !ok {
		available := make([]string, 0, len(m.providers))
		for k := range m.providers {
			available = append(available, k)
		}
		return "", fmt.Errorf("unsupported secret provider %q (available: %s)", provider, strings.Join(available, ", "))
	}
	return r.ResolveSecret(ctx, provider, ref, fragment)
}
