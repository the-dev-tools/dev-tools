using DevTools;

namespace Api.Environment;

@TanStackDB.collection
model Environment {
  @primaryKey environmentId: Id;
  @foreignKey @removeVisibility(Lifecycle.Update) workspaceId: Id;
  name: string;
  description: string;
  @visibility(Lifecycle.Read) updated?: Protobuf.WellKnown.Timestamp;
  @visibility(Lifecycle.Read) isGlobal: boolean;
  order: float32;
}

@TanStackDB.collection
model EnvironmentVariable {
  @primaryKey environmentVariableId: Id;
  ...CommonTableFields<Environment>;
}
