import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

import "./environment.tsp";
import "./resource.tsp";
import "./flow-item/node.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/flow/v1;flowv1",
  },
})
namespace API.Flow;

@autoFields
model Flow {
  @key flowID: Resource.ID;
  name: string;
}

model FlowListItem is Resource.List.Item<Flow>;
model FlowListRequest is Resource.List.Request<Flow>;
model FlowListResponse is Resource.List.Response<FlowItem.Node.Node>;
model FlowGetRequest is Resource.Get.Request<Flow>;
model FlowGetResponse is Resource.Get.Response<Flow>;
model FlowCreateRequest is Resource.Create.Request<Flow>;
model FlowCreateResponse is Resource.Create.Response<Flow>;
model FlowUpdateRequest is Resource.Update.Request<Flow>;
model FlowUpdateResponse is Resource.Update.Response<Flow>;
model FlowDeleteRequest is Resource.Delete.Request<Flow>;
model FlowDeleteResponse is Resource.Delete.Response<Flow>;

@Protobuf.service
interface FlowService {
  FlowList(...FlowListRequest): FlowListResponse;
  FlowGet(...FlowGetRequest): FlowGetResponse;
  FlowCreate(...FlowCreateRequest): FlowCreateResponse;
  FlowUpdate(...FlowUpdateRequest): FlowUpdateResponse;
  FlowDelete(...FlowDeleteRequest): FlowDeleteResponse;
}
