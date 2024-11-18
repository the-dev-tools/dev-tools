import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

import "./workspace.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "tag.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/tag/v1;tagv1",
  },
})
namespace API.Tag;

@autoFields
@parentResource(Workspace.Workspace)
model FlowTag {
  @key TagID: Resource.ID;
  name: string;
}

model FlowTagListItem is Resource.List.Item<FlowTag>;
model FlowTagListRequest is Resource.List.Request<FlowTag>;
model FlowTagistResponse is Resource.List.Response<FlowTagListItem>;
model FlowTagetRequest is Resource.Get.Request<FlowTag>;
model FlowTagetResponse is Resource.Get.Response<FlowTag>;
model FlowTagreateRequest is Resource.Create.Request<FlowTag>;
model FlowTagreateResponse is Resource.Create.Response<FlowTag>;
model FlowTagpdateRequest is Resource.Update.Request<FlowTag>;
model FlowTagpdateResponse is Resource.Update.Response<FlowTag>;
model FlowTageleteRequest is Resource.Delete.Request<FlowTag>;
model FlowTageleteResponse is Resource.Delete.Response<FlowTag>;

