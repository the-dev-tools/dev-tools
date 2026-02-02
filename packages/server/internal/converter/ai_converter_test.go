package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"

	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	filev1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/file_system/v1"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestToAPIFileKind_Credential(t *testing.T) {
	assert.Equal(t, filev1.FileKind_FILE_KIND_CREDENTIAL, ToAPIFileKind(mfile.ContentTypeCredential))
}

func TestToAPINodeKind_Ai(t *testing.T) {
	assert.Equal(t, flowv1.NodeKind_NODE_KIND_AI, ToAPINodeKind(mflow.NODE_KIND_AI))
}

func TestToAPICredential(t *testing.T) {
	id := idwrap.NewNow()
	cred := mcredential.Credential{
		ID:   id,
		Name: "Test OpenAI",
		Kind: mcredential.CREDENTIAL_KIND_OPENAI,
	}

	apiCred := ToAPICredential(cred)
	assert.Equal(t, id.Bytes(), apiCred.CredentialId)
	assert.Equal(t, "Test OpenAI", apiCred.Name)
	assert.Equal(t, credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI, apiCred.Kind)
}
