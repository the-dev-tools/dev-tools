import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../resource.tsp";
import "../flow.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.edge.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/edge/v1;edgev1",
  },
})
namespace API.FlowItem.Edge;

@parentResource(Flow.Flow)
@autoFields
model Edge {
  edgeId: Resource.ID;
  sourceId: Resource.ID;
  targetId: Resource.ID;
}

model EdgeListItem is Resource.List.Item<Edge>;
model EdgeListRequest is Resource.List.Request<Edge>;
model EdgeListResponse is Resource.List.Response<EdgeListItem>;
model EdgeGetRequest is Resource.Get.Request<Edge>;
model EdgeGetResponse is Resource.Get.Response<Edge>;
model EdgeCreateRequest is Resource.Create.Request<Edge>;
model EdgeCreateResponse is Resource.Create.Response<Edge>;
model EdgeUpdateRequest is Resource.Update.Request<Edge>;
model EdgeUpdateResponse is Resource.Update.Response<Edge>;
model EdgeDeleteRequest is Resource.Delete.Request<Edge>;
model EdgeDeleteResponse is Resource.Delete.Response<Edge>;

@Protobuf.service
interface EdgeService {
  EdgeList(...EdgeListRequest): EdgeListResponse;
  EdgeGet(...EdgeGetRequest): EdgeGetResponse;
  EdgeCreate(...EdgeCreateRequest): EdgeCreateResponse;
  EdgeUpdate(...EdgeUpdateRequest): EdgeUpdateResponse;
  EdgeDelete(...EdgeDeleteRequest): EdgeDeleteResponse;
}
