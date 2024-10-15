import "@typespec/protobuf";
import "@typespec/rest";

import "./environment.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "variable.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/variable/v1;variablev1",
  },
})
namespace API.Variable;

@parentResource(Environment.Environment)
model Variable {
  @field(3) @key variableId: Resource.ID;
  @field(4) name: string;
  @field(5) value: string;
  @field(6) enabled: boolean;
  @field(7) description: string;
}

model ListVariablesItem is Resource.List.Item<Variable>;
model ListVariablesRequest is Resource.List.Request<Variable>;
model ListVariablesResponse is Resource.List.Response<ListVariablesItem>;
model GetVariableRequest is Resource.Get.Request<Variable>;
model GetVariableResponse is Resource.Get.Response<Variable>;
model CreateVariableRequest is Resource.Create.Request<Variable>;
model CreateVariableResponse is Resource.Create.Response<Variable>;
model UpdateVariableRequest is Resource.Update.Request<Variable>;
model UpdateVariableResponse is Resource.Update.Response<Variable>;
model DeleteVariableRequest is Resource.Delete.Request<Variable>;
model DeleteVariableResponse is Resource.Delete.Response<Variable>;

@Protobuf.service
interface VariableService {
  ListVariables(...ListVariablesRequest): ListVariablesResponse;
  GetVariable(...GetVariableRequest): GetVariableResponse;
  CreateVariable(...CreateVariableRequest): CreateVariableResponse;
  UpdateVariable(...UpdateVariableRequest): UpdateVariableResponse;
  DeleteVariable(...DeleteVariableRequest): DeleteVariableResponse;
}
