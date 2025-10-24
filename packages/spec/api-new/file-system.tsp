using DevTools;

namespace Api.FileSystem;

enum FileKind {
  Folder,
  Http,
  Flow,
}

@TanStackDB.collection
model File {
  @key fileId: Id;
  @visibility(Lifecycle.Read) workspaceId: Id;
  folderId?: Id;
  @removeVisibility(Lifecycle.Update) kind: FileKind;
  @removeVisibility(Lifecycle.Update) contentId: Id;
  name: string;
  order: float32;
}
