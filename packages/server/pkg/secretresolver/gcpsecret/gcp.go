// Package gcpsecret implements SecretResolver for Google Cloud Secret Manager.
package gcpsecret

import (
	"context"
	"fmt"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/secretresolver"
)

type cacheEntry struct {
	value     string
	fetchedAt time.Time
}

// Resolver resolves secrets from Google Cloud Secret Manager.
// It uses Application Default Credentials (ADC) for authentication.
type Resolver struct {
	client *secretmanager.Client
	cache  sync.Map // map[string]cacheEntry
	ttl    time.Duration
}

// New creates a new GCP secret resolver.
func New(ctx context.Context, opts ...Option) (*Resolver, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCP Secret Manager client: %w", err)
	}

	return &Resolver{client: client, ttl: cfg.cacheTTL}, nil
}

// ResolveSecret fetches a secret from GCP Secret Manager.
func (r *Resolver) ResolveSecret(ctx context.Context, provider, ref, fragment string) (string, error) {
	if provider != "gcp" {
		return "", fmt.Errorf("GCP resolver does not support provider %q", provider)
	}

	cacheKey := ref + "#" + fragment
	if entry, ok := r.cache.Load(cacheKey); ok {
		e := entry.(cacheEntry)
		if time.Since(e.fetchedAt) < r.ttl {
			return e.value, nil
		}
	}

	result, err := r.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: ref,
	})
	if err != nil {
		return "", fmt.Errorf("accessing GCP secret %q: %w", ref, err)
	}

	raw := string(result.Payload.Data)

	value, err := secretresolver.ExtractFragment(raw, fragment)
	if err != nil {
		return "", fmt.Errorf("extracting fragment from GCP secret %q: %w", ref, err)
	}

	r.cache.Store(cacheKey, cacheEntry{value: value, fetchedAt: time.Now()})
	return value, nil
}

// Close releases the underlying gRPC connection.
func (r *Resolver) Close() error {
	return r.client.Close()
}

type config struct {
	cacheTTL time.Duration
}

func defaultConfig() config {
	return config{cacheTTL: 5 * time.Minute}
}

// Option configures the GCP resolver.
type Option func(*config)

// WithCacheTTL sets the cache TTL for resolved secrets.
func WithCacheTTL(d time.Duration) Option {
	return func(c *config) { c.cacheTTL = d }
}
