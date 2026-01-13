package scredential

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

func TestCredentialMapper(t *testing.T) {
	id := idwrap.NewNow()
	wsID := idwrap.NewNow()

	mc := mcredential.Credential{
		ID:          id,
		WorkspaceID: wsID,
		Name:        "Test Cred",
		Kind:        mcredential.CREDENTIAL_KIND_OPENAI,
	}

	dbc := ConvertCredentialToDB(mc)
	assert.Equal(t, id, dbc.ID)
	assert.Equal(t, wsID, dbc.WorkspaceID)
	assert.Equal(t, "Test Cred", dbc.Name)
	assert.Equal(t, int8(0), dbc.Kind)

	mc2 := ConvertDBToCredential(dbc)
	assert.Equal(t, mc.ID, mc2.ID)
	assert.Equal(t, mc.Name, mc2.Name)
	assert.Equal(t, mc.Kind, mc2.Kind)
}

func TestCredentialOpenAIMapper(t *testing.T) {
	id := idwrap.NewNow()
	baseUrl := "https://api.openai.com"

	mo := mcredential.CredentialOpenAI{
		CredentialID:   id,
		Token:          "sk-123",
		BaseUrl:        &baseUrl,
		EncryptionType: credvault.EncryptionNone,
	}

	// Simulate plaintext (no encryption)
	tokenBytes := []byte(mo.Token)
	dbo := ConvertCredentialOpenAIToDB(mo, tokenBytes)
	assert.Equal(t, id, dbo.CredentialID)
	assert.Equal(t, tokenBytes, dbo.Token)
	assert.True(t, dbo.BaseUrl.Valid)
	assert.Equal(t, baseUrl, dbo.BaseUrl.String)
	assert.Equal(t, int8(credvault.EncryptionNone), dbo.EncryptionType)

	// Test DB to Model conversion
	dbRow := gen.CredentialOpenai{
		CredentialID:   id,
		Token:          []byte("sk-456"),
		BaseUrl:        sql.NullString{String: baseUrl, Valid: true},
		EncryptionType: int8(credvault.EncryptionNone),
	}
	mo2, rawToken := ConvertDBToCredentialOpenAIRaw(dbRow)
	assert.Equal(t, id, mo2.CredentialID)
	assert.Equal(t, []byte("sk-456"), rawToken)
	assert.Equal(t, baseUrl, *mo2.BaseUrl)
	assert.Equal(t, credvault.EncryptionNone, mo2.EncryptionType)
}
