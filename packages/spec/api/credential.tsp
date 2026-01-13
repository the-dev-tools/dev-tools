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

model CredentialOpenAi {
  @primaryKey credentialId: Id;
  token: string;
  baseUrl?: string;
}

model GetCredentialOpenAiRequest {
  credentialId: Id;
}

model GetCredentialOpenAiResponse {
  ...CredentialOpenAi;
}

op GetCredentialOpenAi(...GetCredentialOpenAiRequest): GetCredentialOpenAiResponse;

model CredentialGemini {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}

model GetCredentialGeminiRequest {
  credentialId: Id;
}

model GetCredentialGeminiResponse {
  ...CredentialGemini;
}

op GetCredentialGemini(...GetCredentialGeminiRequest): GetCredentialGeminiResponse;

model CredentialAnthropic {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}

model GetCredentialAnthropicRequest {
  credentialId: Id;
}

model GetCredentialAnthropicResponse {
  ...CredentialAnthropic;
}

op GetCredentialAnthropic(...GetCredentialAnthropicRequest): GetCredentialAnthropicResponse;
