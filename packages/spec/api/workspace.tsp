using DevTools;

namespace Api.Workspace;

@TanStackDB.collection
model Workspace {
  @primaryKey workspaceId: Id;
  @foreignKey selectedEnvironmentId: Id;
  name: string;
  @visibility(Lifecycle.Read) updated?: Protobuf.WellKnown.Timestamp;
  order: float32;
  syncPath?: string;
  syncFormat?: string;
  syncEnabled: boolean;
}
