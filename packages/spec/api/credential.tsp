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

op GetCredentialOpenAi(...GetCredentialOpenAiRequest): CredentialOpenAi;

model CredentialGemini {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}

model GetCredentialGeminiRequest {
  credentialId: Id;
}

op GetCredentialGemini(...GetCredentialGeminiRequest): CredentialGemini;

model CredentialAnthropic {
  @primaryKey credentialId: Id;
  apiKey: string;
  baseUrl?: string;
}

model GetCredentialAnthropicRequest {
  credentialId: Id;
}

op GetCredentialAnthropic(...GetCredentialAnthropicRequest): CredentialAnthropic;
