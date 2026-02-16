using DevTools;

namespace Api.FileSystem;

enum FileKind {
  Folder,
  Http,
  HttpDelta,
  Flow,
  Credential,
  GraphQL,
}

@TanStackDB.collection
model File {
  @primaryKey fileId: Id;
  @foreignKey @removeVisibility(Lifecycle.Update) workspaceId: Id;
  @foreignKey parentId?: Id;
  @removeVisibility(Lifecycle.Update) kind: FileKind;
  order: float32;
}

@TanStackDB.collection
model Folder {
  @primaryKey folderId: Id;
  name: string;
}
