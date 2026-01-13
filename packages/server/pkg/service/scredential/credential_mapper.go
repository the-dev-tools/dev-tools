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

// ConvertDBToCredentialOpenAIRaw converts DB row to model with raw secret (bytes).
// Caller is responsible for decryption using the EncryptionType.
func ConvertDBToCredentialOpenAIRaw(c gen.CredentialOpenai) (*mcredential.CredentialOpenAI, []byte) {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialOpenAI{
		CredentialID:   c.CredentialID,
		Token:          "", // Will be set by caller after decryption
		BaseUrl:        baseUrl,
		EncryptionType: c.EncryptionType,
	}, c.Token
}

// ConvertCredentialOpenAIToDB converts model to DB row with encrypted secret.
// Caller is responsible for encrypting the secret before calling.
func ConvertCredentialOpenAIToDB(mc mcredential.CredentialOpenAI, encryptedToken []byte) gen.CredentialOpenai {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialOpenai{
		CredentialID:   mc.CredentialID,
		Token:          encryptedToken,
		BaseUrl:        baseUrl,
		EncryptionType: mc.EncryptionType,
	}
}

// ConvertDBToCredentialGeminiRaw converts DB row to model with raw secret (bytes).
func ConvertDBToCredentialGeminiRaw(c gen.CredentialGemini) (*mcredential.CredentialGemini, []byte) {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialGemini{
		CredentialID:   c.CredentialID,
		ApiKey:         "", // Will be set by caller after decryption
		BaseUrl:        baseUrl,
		EncryptionType: c.EncryptionType,
	}, c.ApiKey
}

// ConvertCredentialGeminiToDB converts model to DB row with encrypted secret.
func ConvertCredentialGeminiToDB(mc mcredential.CredentialGemini, encryptedKey []byte) gen.CredentialGemini {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialGemini{
		CredentialID:   mc.CredentialID,
		ApiKey:         encryptedKey,
		BaseUrl:        baseUrl,
		EncryptionType: mc.EncryptionType,
	}
}

// ConvertDBToCredentialAnthropicRaw converts DB row to model with raw secret (bytes).
func ConvertDBToCredentialAnthropicRaw(c gen.CredentialAnthropic) (*mcredential.CredentialAnthropic, []byte) {
	var baseUrl *string
	if c.BaseUrl.Valid {
		baseUrl = &c.BaseUrl.String
	}

	return &mcredential.CredentialAnthropic{
		CredentialID:   c.CredentialID,
		ApiKey:         "", // Will be set by caller after decryption
		BaseUrl:        baseUrl,
		EncryptionType: c.EncryptionType,
	}, c.ApiKey
}

// ConvertCredentialAnthropicToDB converts model to DB row with encrypted secret.
func ConvertCredentialAnthropicToDB(mc mcredential.CredentialAnthropic, encryptedKey []byte) gen.CredentialAnthropic {
	var baseUrl sql.NullString
	if mc.BaseUrl != nil {
		baseUrl = sql.NullString{String: *mc.BaseUrl, Valid: true}
	}

	return gen.CredentialAnthropic{
		CredentialID:   mc.CredentialID,
		ApiKey:         encryptedKey,
		BaseUrl:        baseUrl,
		EncryptionType: mc.EncryptionType,
	}
}
