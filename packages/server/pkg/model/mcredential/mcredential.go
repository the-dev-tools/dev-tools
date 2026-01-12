package mcredential

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type CredentialKind int8

const (
	CREDENTIAL_KIND_OPENAI    CredentialKind = 0
	CREDENTIAL_KIND_GEMINI    CredentialKind = 1
	CREDENTIAL_KIND_ANTHROPIC CredentialKind = 2
)

type Credential struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	Kind        CredentialKind
}

type CredentialOpenAI struct {
	CredentialID idwrap.IDWrap
	Token        string
	BaseUrl      *string
}

type CredentialGemini struct {
	CredentialID idwrap.IDWrap
	ApiKey       string
	BaseUrl      *string
}

type CredentialAnthropic struct {
	CredentialID idwrap.IDWrap
	ApiKey       string
	BaseUrl      *string
}
