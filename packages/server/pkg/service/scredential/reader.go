package scredential

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

// Decrypter handles secret decryption. Implemented by credvault.Vault.
type Decrypter interface {
	DecryptString(ciphertext []byte, encType credvault.EncryptionType) (string, error)
}

// ReaderOption configures a CredentialReader.
type ReaderOption func(*CredentialReader)

// WithDecrypter sets the decrypter for automatic secret decryption.
func WithDecrypter(d Decrypter) ReaderOption {
	return func(r *CredentialReader) {
		r.decrypter = d
	}
}

// CredentialReader reads credentials from the database.
type CredentialReader struct {
	queries   *gen.Queries
	decrypter Decrypter
}

// NewCredentialReader creates a new reader with the given options.
func NewCredentialReader(db *sql.DB, opts ...ReaderOption) *CredentialReader {
	r := &CredentialReader{
		queries: gen.New(db),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// NewCredentialReaderFromQueries creates a reader from existing queries.
func NewCredentialReaderFromQueries(queries *gen.Queries, opts ...ReaderOption) *CredentialReader {
	r := &CredentialReader{
		queries: queries,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *CredentialReader) GetCredential(ctx context.Context, id idwrap.IDWrap) (*mcredential.Credential, error) {
	cred, err := r.queries.GetCredential(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToCredential(cred), nil
}

func (r *CredentialReader) GetCredentialOpenAI(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialOpenAI, error) {
	dbCred, err := r.queries.GetCredentialOpenAI(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	result, encryptedToken := ConvertDBToCredentialOpenAIRaw(dbCred)

	token, err := r.decryptSecret(encryptedToken, result.EncryptionType)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	result.Token = token

	return result, nil
}

func (r *CredentialReader) GetCredentialGemini(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialGemini, error) {
	dbCred, err := r.queries.GetCredentialGemini(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	result, encryptedKey := ConvertDBToCredentialGeminiRaw(dbCred)

	apiKey, err := r.decryptSecret(encryptedKey, result.EncryptionType)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	result.ApiKey = apiKey

	return result, nil
}

func (r *CredentialReader) GetCredentialAnthropic(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialAnthropic, error) {
	dbCred, err := r.queries.GetCredentialAnthropic(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	result, encryptedKey := ConvertDBToCredentialAnthropicRaw(dbCred)

	apiKey, err := r.decryptSecret(encryptedKey, result.EncryptionType)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	result.ApiKey = apiKey

	return result, nil
}

func (r *CredentialReader) ListCredentials(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcredential.Credential, error) {
	creds, err := r.queries.GetCredentialsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mcredential.Credential{}, nil
		}
		return nil, err
	}

	result := make([]mcredential.Credential, len(creds))
	for i, c := range creds {
		result[i] = *ConvertDBToCredential(c)
	}
	return result, nil
}

// decryptSecret decrypts or returns plaintext based on encryption type.
func (r *CredentialReader) decryptSecret(ciphertext []byte, encType credvault.EncryptionType) (string, error) {
	if encType == credvault.EncryptionNone {
		return string(ciphertext), nil
	}
	if r.decrypter == nil {
		return "", errors.New("decrypter not configured but secret is encrypted")
	}
	return r.decrypter.DecryptString(ciphertext, encType)
}
