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

model VariableListItem is Resource.List.Item<Variable>;
model VariableListRequest is Resource.List.Request<Variable>;
model VariableListResponse is Resource.List.Response<VariableListItem>;
model VariableGetRequest is Resource.Get.Request<Variable>;
model VariableGetResponse is Resource.Get.Response<Variable>;
model VariableCreateRequest is Resource.Create.Request<Variable>;
model VariableCreateResponse is Resource.Create.Response<Variable>;
model VariableUpdateRequest is Resource.Update.Request<Variable>;
model VariableUpdateResponse is Resource.Update.Response<Variable>;
model VariableDeleteRequest is Resource.Delete.Request<Variable>;
model VariableDeleteResponse is Resource.Delete.Response<Variable>;

@Protobuf.service
interface VariableService {
  VariableList(...VariableListRequest): VariableListResponse;
  VariableGet(...VariableGetRequest): VariableGetResponse;
  VariableCreate(...VariableCreateRequest): VariableCreateResponse;
  VariableUpdate(...VariableUpdateRequest): VariableUpdateResponse;
  VariableDelete(...VariableDeleteRequest): VariableDeleteResponse;
}
