using DevTools;

namespace Api.Environment;

@TanStackDB.collection
model Environment {
  @key environmentId: Id;
  @removeVisibility(Lifecycle.Update) workspaceId: Id;
  name: string;
  description: string;
  @visibility(Lifecycle.Read) updated?: Protobuf.WellKnown.Timestamp;
  @visibility(Lifecycle.Read) isGlobal: boolean;
  order: float32;
}

@TanStackDB.collection
model EnvironmentVariable {
  @key environmentVariableId: Id;
  environmentId: Id;
  ...CommonTableFields;
}
