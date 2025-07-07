import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "environment.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/environment/v1;environmentv1",
  },
})
namespace API.Environment;

@autoFields
@normalize
@entity
@parentResource(Workspace.Workspace)
model Environment {
  @key environmentId: Resource.Id;
  name: string;
  description: string;
  @visibility(Resource.Mutation.None) updated: WellKnown.Timestamp;
  @visibility(Resource.Mutation.None) isGlobal: boolean;
}

@autoFields
model EnvironmentMoveRequest {
  ...ParentKeyOf<Environment>;
  ...KeyOf<Environment>;
  position: Resource.MovePosition;
  targetEnvironmentId: Resource.Id;
}

model EnvironmentMoveResponse {}

@Protobuf.service
interface EnvironmentService extends Resource.CRUD<Environment> {
  @endpoint("environment.js:move")
  EnvironmentMove(...EnvironmentMoveRequest): EnvironmentMoveResponse;
}
