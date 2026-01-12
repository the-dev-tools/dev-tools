package scredential

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

type CredentialWriter struct {
	queries *gen.Queries
}

func NewCredentialWriterFromQueries(queries *gen.Queries) *CredentialWriter {
	return &CredentialWriter{
		queries: queries,
	}
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
	return w.queries.CreateCredentialOpenAI(ctx, gen.CreateCredentialOpenAIParams{
		CredentialID: cred.CredentialID,
		Token:        cred.Token,
		BaseUrl:      baseUrl,
	})
}

func (w *CredentialWriter) CreateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}
	return w.queries.CreateCredentialGemini(ctx, gen.CreateCredentialGeminiParams{
		CredentialID: cred.CredentialID,
		ApiKey:       cred.ApiKey,
		BaseUrl:      baseUrl,
	})
}

func (w *CredentialWriter) CreateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}
	return w.queries.CreateCredentialAnthropic(ctx, gen.CreateCredentialAnthropicParams{
		CredentialID: cred.CredentialID,
		ApiKey:       cred.ApiKey,
		BaseUrl:      baseUrl,
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
	return w.queries.UpdateCredentialOpenAI(ctx, gen.UpdateCredentialOpenAIParams{
		CredentialID: cred.CredentialID,
		Token:        cred.Token,
		BaseUrl:      baseUrl,
	})
}

func (w *CredentialWriter) UpdateCredentialGemini(ctx context.Context, cred *mcredential.CredentialGemini) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}
	return w.queries.UpdateCredentialGemini(ctx, gen.UpdateCredentialGeminiParams{
		CredentialID: cred.CredentialID,
		ApiKey:       cred.ApiKey,
		BaseUrl:      baseUrl,
	})
}

func (w *CredentialWriter) UpdateCredentialAnthropic(ctx context.Context, cred *mcredential.CredentialAnthropic) error {
	var baseUrl sql.NullString
	if cred.BaseUrl != nil {
		baseUrl = sql.NullString{String: *cred.BaseUrl, Valid: true}
	}
	return w.queries.UpdateCredentialAnthropic(ctx, gen.UpdateCredentialAnthropicParams{
		CredentialID: cred.CredentialID,
		ApiKey:       cred.ApiKey,
		BaseUrl:      baseUrl,
	})
}

func (w *CredentialWriter) DeleteCredential(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteCredential(ctx, id)
}
