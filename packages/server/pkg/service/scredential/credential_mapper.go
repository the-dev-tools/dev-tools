package scredential

import (
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

func ConvertDBToCredential(c gen.Credential) *mcredential.Credential {
	return &mcredential.Credential{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Kind:        mcredential.CredentialKind(c.Kind),
	}
}

func ConvertCredentialToDB(mc mcredential.Credential) gen.Credential {
	return gen.Credential{
		ID:          mc.ID,
		WorkspaceID: mc.WorkspaceID,
		Name:        mc.Name,
		Kind:        int8(mc.Kind),
	}
}

func ConvertDBToCredentialOpenAI(c gen.CredentialOpenai) *mcredential.CredentialOpenAI {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialOpenAI{
		CredentialID: c.CredentialID,
		Token:        c.Token,
		BaseUrl:      baseUrl,
	}
}

func ConvertCredentialOpenAIToDB(mc mcredential.CredentialOpenAI) gen.CredentialOpenai {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialOpenai{
		CredentialID: mc.CredentialID,
		Token:        mc.Token,
		BaseUrl:      baseUrl,
	}
}

func ConvertDBToCredentialGemini(c gen.CredentialGemini) *mcredential.CredentialGemini {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialGemini{
		CredentialID: c.CredentialID,
		ApiKey:       c.ApiKey,
		BaseUrl:      baseUrl,
	}
}

func ConvertCredentialGeminiToDB(mc mcredential.CredentialGemini) gen.CredentialGemini {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialGemini{
		CredentialID: mc.CredentialID,
		ApiKey:       mc.ApiKey,
		BaseUrl:      baseUrl,
	}
}

func ConvertDBToCredentialAnthropic(c gen.CredentialAnthropic) *mcredential.CredentialAnthropic {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialAnthropic{
		CredentialID: c.CredentialID,
		ApiKey:       c.ApiKey,
		BaseUrl:      baseUrl,
	}
}

func ConvertCredentialAnthropicToDB(mc mcredential.CredentialAnthropic) gen.CredentialAnthropic {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialAnthropic{
		CredentialID: mc.CredentialID,
		ApiKey:       mc.ApiKey,
		BaseUrl:      baseUrl,
	}
}
