package scredential

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

type CredentialReader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewCredentialReader(db *sql.DB, logger *slog.Logger) *CredentialReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &CredentialReader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewCredentialReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *CredentialReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &CredentialReader{
		queries: queries,
		logger:  logger,
	}
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
	cred, err := r.queries.GetCredentialOpenAI(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToCredentialOpenAI(cred), nil
}

func (r *CredentialReader) GetCredentialGemini(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialGemini, error) {
	cred, err := r.queries.GetCredentialGemini(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToCredentialGemini(cred), nil
}

func (r *CredentialReader) GetCredentialAnthropic(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialAnthropic, error) {
	cred, err := r.queries.GetCredentialAnthropic(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToCredentialAnthropic(cred), nil
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
