package scredential

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		CredentialID: id,
		Token:        "sk-123",
		BaseUrl:      &baseUrl,
	}

	dbo := ConvertCredentialOpenAIToDB(mo)
	assert.Equal(t, id, dbo.CredentialID)
	assert.Equal(t, "sk-123", dbo.Token)
	assert.True(t, dbo.BaseUrl.Valid)
	assert.Equal(t, baseUrl, dbo.BaseUrl.String)

	mo2 := ConvertDBToCredentialOpenAI(dbo)
	assert.Equal(t, mo.CredentialID, mo2.CredentialID)
	assert.Equal(t, mo.Token, mo2.Token)
	assert.Equal(t, *mo.BaseUrl, *mo2.BaseUrl)
}
