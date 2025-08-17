using DevTools;

namespace API.Environment;

@DataClient.entity
@parent(Workspace.Workspace)
model Environment {
  @key environmentId: Resource.Id;
  name: string;
  description: string;
  @visibility(Resource.Mutation.None) updated: Protobuf.WellKnown.Timestamp;
  @visibility(Resource.Mutation.None) isGlobal: boolean;
}

model EnvironmentMoveRequest {
  ...ParentKeyOf<Environment>;
  ...KeyOf<Environment>;
  position: Resource.MovePosition;
  targetEnvironmentId: Resource.Id;
}

model EnvironmentMoveResponse {}

interface EnvironmentService extends Resource.CRUD<Environment> {
  @DataClient.endpoint("environment.js:move")
  EnvironmentMove(...EnvironmentMoveRequest): EnvironmentMoveResponse;
}
