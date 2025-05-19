import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./environment.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "variable.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/variable/v1;variablev1",
  },
})
namespace API.Variable;

@autoFields
@normalize
@entity
@parentResource(Environment.Environment)
model Variable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model VariableChange is Resource.Change<Variable>;
model VariableCreateRequest is Resource.Create.Request<Variable> {
  @autoField @normalKey workspaceId?: Resource.Id;
}
model VariableListRequest is Resource.List.Request<Variable> {
  @autoField @normalKey workspaceId?: Resource.Id;
}

@Protobuf.service
interface VariableService
  extends Resource.CRUD<Variable, TCreateRequest = VariableCreateRequest, TListRequest = VariableListRequest> {}
