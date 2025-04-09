import "@typespec/protobuf";

import "../lib";

import "./resource.tsp";
import "./flow.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flowvariable.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flowvariable/v1;flowvariablev1",
  },
})
namespace API.FlowVariable;

@autoFields
@normalize
@parentResource(Flow.Flow)
model FlowVariable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model FlowVariableChange is Resource.Change<FlowVariable>;
model FlowVariableListItem is Resource.List.Item<FlowVariable>;
model FlowVariableListRequest is Resource.List.Request<FlowVariable>;
model FlowVariableListResponse is Resource.List.Response<FlowVariable, FlowVariableListItem>;
model FlowVariableGetRequest is Resource.Get.Request<FlowVariable>;
model FlowVariableGetResponse is Resource.Get.Response<FlowVariable>;
model FlowVariableCreateRequest is Resource.Create.Request<FlowVariable>;
model FlowVariableCreateResponse is Resource.Create.Response<FlowVariable, FlowVariableListResponse>;
model FlowVariableUpdateRequest is Resource.Update.Request<FlowVariable>;
model FlowVariableUpdateResponse is Resource.Update.Response<FlowVariable>;
model FlowVariableDeleteRequest is Resource.Delete.Request<FlowVariable>;
model FlowVariableDeleteResponse is Resource.Delete.Response<FlowVariable>;

@Protobuf.service
interface FlowVariableService {
  FlowVariableList(...FlowVariableListRequest): FlowVariableListResponse;
  FlowVariableGet(...FlowVariableGetRequest): FlowVariableGetResponse;
  FlowVariableCreate(...FlowVariableCreateRequest): FlowVariableCreateResponse;
  FlowVariableUpdate(...FlowVariableUpdateRequest): FlowVariableUpdateResponse;
  FlowVariableDelete(...FlowVariableDeleteRequest): FlowVariableDeleteResponse;
}
