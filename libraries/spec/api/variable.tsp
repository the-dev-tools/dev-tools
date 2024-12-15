import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

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
@parentResource(Environment.Environment)
model Variable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model VariableListItem is Resource.List.Item<Variable> {
  ...ParentKeyOf<Variable>;
}
model VariableListRequest is Resource.List.Request<Variable> {
  @autoField @visibility("list") workspaceId?: Resource.Id;
}
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
