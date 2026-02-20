using DevTools;

namespace Api.User;

enum AuthProvider {
  Email,
  Google,
}

@TanStackDB.collection(#{ canCreate: false, canDelete: false })
model User {
  @primaryKey userId: Id;
  @removeVisibility(Lifecycle.Update) email: string;
  name: string;
  image?: string;
}

@TanStackDB.collection(#{ isReadOnly: true })
model LinkedAccount {
  @primaryKey accountId: Id;
  @foreignKey userId: Id;
  provider: AuthProvider;
}
