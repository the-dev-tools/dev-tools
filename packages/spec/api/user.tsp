using DevTools;
using Api.AuthCommon;

namespace Api.User;

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
