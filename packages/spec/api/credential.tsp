using DevTools;

namespace Api.Credential;

enum CredentialKind {
  OpenAi,
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
