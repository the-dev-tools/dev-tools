package scredential

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

// ServiceOption configures a CredentialService.
type ServiceOption func(*CredentialService)

// WithVault configures the service to use vault for encryption/decryption.
func WithVault(v *credvault.Vault) ServiceOption {
	return func(s *CredentialService) {
		s.vault = v
	}
}

// CredentialService provides credential CRUD with encryption.
// By default uses XChaCha20-Poly1305 with a static key (obfuscation).
type CredentialService struct {
	queries *gen.Queries
	vault   *credvault.Vault
}

// NewCredentialService creates a new service with default encryption.
// Override with WithVault(nil) for plaintext or WithVault(customVault) for secure encryption.
func NewCredentialService(queries *gen.Queries, opts ...ServiceOption) CredentialService {
	s := CredentialService{
		queries: queries,
		vault:   credvault.NewDefault(), // Default: encrypt with static key
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

// TX returns a new service scoped to the transaction.
func (s CredentialService) TX(tx *sql.Tx) CredentialService {
	if tx == nil {
		return s
	}
	return CredentialService{
		queries: s.queries.WithTx(tx),
		vault:   s.vault,
	}
}

// Reader returns a configured credential reader.
func (s CredentialService) Reader() *CredentialReader {
	var opts []ReaderOption
	if s.vault != nil {
		opts = append(opts, WithDecrypter(s.vault))
	}
	return NewCredentialReaderFromQueries(s.queries, opts...)
}

// writer returns a configured credential writer.
func (s CredentialService) writer() *CredentialWriter {
	var opts []WriterOption
	if s.vault != nil {
		opts = append(opts, WithEncrypter(s.vault))
	}
	return NewCredentialWriterFromQueries(s.queries, opts...)
}

func (s CredentialService) GetCredential(ctx context.Context, id idwrap.IDWrap) (*mcredential.Credential, error) {
	return s.Reader().GetCredential(ctx, id)
}

func (s CredentialService) GetCredentialOpenAI(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialOpenAI, error) {
	return s.Reader().GetCredentialOpenAI(ctx, id)
}

func (s CredentialService) GetCredentialGemini(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialGemini, error) {
	return s.Reader().GetCredentialGemini(ctx, id)
}

func (s CredentialService) GetCredentialAnthropic(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialAnthropic, error) {
	return s.Reader().GetCredentialAnthropic(ctx, id)
}

func (s CredentialService) ListCredentials(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcredential.Credential, error) {
	return s.Reader().ListCredentials(ctx, workspaceID)
}

func (s CredentialService) CreateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return s.writer().CreateCredential(ctx, cred)
}

func (s CredentialService) CreateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	return s.writer().CreateCredentialOpenAI(ctx, cred)
}

func (s CredentialService) CreateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	return s.writer().CreateCredentialGemini(ctx, cred)
}

func (s CredentialService) CreateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	return s.writer().CreateCredentialAnthropic(ctx, cred)
}

func (s CredentialService) UpdateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return s.writer().UpdateCredential(ctx, cred)
}

func (s CredentialService) UpdateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	return s.writer().UpdateCredentialOpenAI(ctx, cred)
}

func (s CredentialService) UpdateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	return s.writer().UpdateCredentialGemini(ctx, cred)
}

func (s CredentialService) UpdateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	return s.writer().UpdateCredentialAnthropic(ctx, cred)
}

func (s CredentialService) DeleteCredential(ctx context.Context, id idwrap.IDWrap) error {
	return s.writer().DeleteCredential(ctx, id)
}
