package scredential

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

// Encrypter handles secret encryption. Implemented by credvault.Vault.
type Encrypter interface {
	Encrypt(plaintext []byte, encType credvault.EncryptionType) ([]byte, error)
}

// WriterOption configures a CredentialWriter.
type WriterOption func(*CredentialWriter)

// WithEncrypter sets the encrypter for automatic secret encryption.
// When set, secrets are encrypted using XChaCha20-Poly1305 by default.
func WithEncrypter(e Encrypter) WriterOption {
	return func(w *CredentialWriter) {
		w.encrypter = e
	}
}

// CredentialWriter writes credentials to the database.
type CredentialWriter struct {
	queries   *gen.Queries
	encrypter Encrypter
}

// NewCredentialWriterFromQueries creates a writer with the given options.
func NewCredentialWriterFromQueries(queries *gen.Queries, opts ...WriterOption) *CredentialWriter {
	w := &CredentialWriter{
		queries: queries,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *CredentialWriter) CreateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return w.queries.CreateCredential(ctx, gen.CreateCredentialParams{
		ID:          cred.ID,
		WorkspaceID: cred.WorkspaceID,
		Name:        cred.Name,
		Kind:        int8(cred.Kind),
	})
}

func (w *CredentialWriter) CreateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	tokenBytes, encType, err := w.encryptSecret([]byte(cred.Token), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	return w.queries.CreateCredentialOpenAI(ctx, gen.CreateCredentialOpenAIParams{
		CredentialID:   cred.CredentialID,
		Token:          tokenBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) CreateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	keyBytes, encType, err := w.encryptSecret([]byte(cred.ApiKey), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	return w.queries.CreateCredentialGemini(ctx, gen.CreateCredentialGeminiParams{
		CredentialID:   cred.CredentialID,
		ApiKey:         keyBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) CreateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	keyBytes, encType, err := w.encryptSecret([]byte(cred.ApiKey), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	return w.queries.CreateCredentialAnthropic(ctx, gen.CreateCredentialAnthropicParams{
		CredentialID:   cred.CredentialID,
		ApiKey:         keyBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) UpdateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return w.queries.UpdateCredential(ctx, gen.UpdateCredentialParams{
		ID:   cred.ID,
		Name: cred.Name,
		Kind: int8(cred.Kind),
	})
}

func (w *CredentialWriter) UpdateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	tokenBytes, encType, err := w.encryptSecret([]byte(cred.Token), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	return w.queries.UpdateCredentialOpenAI(ctx, gen.UpdateCredentialOpenAIParams{
		CredentialID:   cred.CredentialID,
		Token:          tokenBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) UpdateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	keyBytes, encType, err := w.encryptSecret([]byte(cred.ApiKey), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	return w.queries.UpdateCredentialGemini(ctx, gen.UpdateCredentialGeminiParams{
		CredentialID:   cred.CredentialID,
		ApiKey:         keyBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) UpdateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}

	keyBytes, encType, err := w.encryptSecret([]byte(cred.ApiKey), cred.EncryptionType)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	return w.queries.UpdateCredentialAnthropic(ctx, gen.UpdateCredentialAnthropicParams{
		CredentialID:   cred.CredentialID,
		ApiKey:         keyBytes,
		BaseUrl:        baseUrl,
		EncryptionType: int8(encType),
	})
}

func (w *CredentialWriter) DeleteCredential(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteCredential(ctx, id)
}

// encryptSecret encrypts if encrypter is set, otherwise stores plaintext.
// Returns the (possibly encrypted) bytes and the effective encryption type.
func (w *CredentialWriter) encryptSecret(plaintext []byte, requestedType credvault.EncryptionType) ([]byte, credvault.EncryptionType, error) {
	if w.encrypter == nil {
		// No encrypter configured - store plaintext
		return plaintext, credvault.EncryptionNone, nil
	}

	// Default to XChaCha20-Poly1305 if caller didn't specify
	encType := requestedType
	if encType == credvault.EncryptionNone {
		encType = credvault.EncryptionXChaCha20Poly1305
	}

	encrypted, err := w.encrypter.Encrypt(plaintext, encType)
	if err != nil {
		return nil, 0, err
	}
	return encrypted, encType, nil
}
