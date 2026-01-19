using DevTools;

namespace Api.Credential;

enum CredentialKind {
  OpenAi,
  Gemini,
  Anthropic,
}

@TanStackDB.collection
model Credential {
  @primaryKey credentialId: Id;
  @foreignKey @visibility(Lifecycle.Create) workspaceId: Id;
  name: string;
  kind: CredentialKind;
}

@TanStackDB.collection
model CredentialOpenAi {
  @primaryKey credentialId: Id;
  token: string;
  baseUrl?: string;
}

@TanStackDB.collection
model CredentialGemini {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}

@TanStackDB.collection
model CredentialAnthropic {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}
