package scredential

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

type CredentialService struct {
	reader  *CredentialReader
	queries *gen.Queries
	logger  *slog.Logger
}

func NewCredentialService(queries *gen.Queries, logger *slog.Logger) CredentialService {
	if logger == nil {
		logger = slog.Default()
	}
	return CredentialService{
		reader:  NewCredentialReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

func (s CredentialService) TX(tx *sql.Tx) CredentialService {
	if tx == nil {
		return s
	}
	newQueries := s.queries.WithTx(tx)
	return CredentialService{
		reader:  NewCredentialReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
		logger:  s.logger,
	}
}

func (s CredentialService) GetCredential(ctx context.Context, id idwrap.IDWrap) (*mcredential.Credential, error) {
	return s.reader.GetCredential(ctx, id)
}

func (s CredentialService) GetCredentialOpenAI(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialOpenAI, error) {
	return s.reader.GetCredentialOpenAI(ctx, id)
}

func (s CredentialService) GetCredentialGemini(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialGemini, error) {
	return s.reader.GetCredentialGemini(ctx, id)
}

func (s CredentialService) GetCredentialAnthropic(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialAnthropic, error) {
	return s.reader.GetCredentialAnthropic(ctx, id)
}

func (s CredentialService) ListCredentials(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcredential.Credential, error) {
	return s.reader.ListCredentials(ctx, workspaceID)
}

func (s CredentialService) CreateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return NewCredentialWriterFromQueries(s.queries).CreateCredential(ctx, cred)
}

func (s CredentialService) CreateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	return NewCredentialWriterFromQueries(s.queries).CreateCredentialOpenAI(ctx, cred)
}

func (s CredentialService) CreateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	return NewCredentialWriterFromQueries(s.queries).CreateCredentialGemini(ctx, cred)
}

func (s CredentialService) CreateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	return NewCredentialWriterFromQueries(s.queries).CreateCredentialAnthropic(ctx, cred)
}

func (s CredentialService) UpdateCredential(ctx context.Context, cred *mcredential.Credential) error {
	return NewCredentialWriterFromQueries(s.queries).UpdateCredential(ctx, cred)
}

func (s CredentialService) UpdateCredentialOpenAI(ctx context.Context, cred *mcredential.CredentialOpenAI) error {
	return NewCredentialWriterFromQueries(s.queries).UpdateCredentialOpenAI(ctx, cred)
}

func (s CredentialService) UpdateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	return NewCredentialWriterFromQueries(s.queries).UpdateCredentialGemini(ctx, cred)
}

func (s CredentialService) UpdateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	return NewCredentialWriterFromQueries(s.queries).UpdateCredentialAnthropic(ctx, cred)
}

func (s CredentialService) DeleteCredential(ctx context.Context, id idwrap.IDWrap) error {
	return NewCredentialWriterFromQueries(s.queries).DeleteCredential(ctx, id)
}

func (s CredentialService) Reader() *CredentialReader { return s.reader }
