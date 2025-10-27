using DevTools;

namespace Api.FileSystem;

enum FileKind {
  Folder,
  Http,
  Flow,
}

@TanStackDB.collection
model File {
  @primaryKey fileId: Id;
  @foreignKey @removeVisibility(Lifecycle.Update) workspaceId: Id;
  @foreignKey folderId?: Id;
  @removeVisibility(Lifecycle.Update) kind: FileKind;
  @foreignKey @removeVisibility(Lifecycle.Update) contentId: Id;
  name: string;
  order: float32;
}
