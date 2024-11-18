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
model Tag {
  @key TagID: Resource.ID;
  name: string;
}

model TagListItem is Resource.List.Item<Tag>;
model TagListRequest is Resource.List.Request<Tag>;
model TagistResponse is Resource.List.Response<TagListItem>;
model TagGetRequest is Resource.Get.Request<Tag>;
model TagTagetResponse is Resource.Get.Response<Tag>;
model TagCreateRequest is Resource.Create.Request<Tag>;
model TagCreateResponse is Resource.Create.Response<Tag>;
model TagUpdateRequest is Resource.Update.Request<Tag>;
model TagUpdateResponse is Resource.Update.Response<Tag>;
model TagDeleteRequest is Resource.Delete.Request<Tag>;
model TagDeleteResponse is Resource.Delete.Response<Tag>;

