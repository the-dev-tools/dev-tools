using DevTools;

namespace Api.Workspace;

@TanStackDB.collection
model Workspace {
  @key workspaceId: Id;
  selectedEnvironmentId: Id;
  name: string;
  @visibility(Lifecycle.Read) updated?: Protobuf.WellKnown.Timestamp;
  order: float32;
}
