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
    go_package: "the-dev-tools/spec/dist/buf/go/tag/v1;tagv1",
  },
})
namespace API.Tag;

enum TagColor {
  TAG_COLOR_UNSPECIFIED: 0,
  TAG_COLOR_SLATE: 1,
  TAG_COLOR_GREEN: 2,
  TAG_COLOR_AMBER: 3,
  TAG_COLOR_SKY: 4,
  TAG_COLOR_PURPLE: 5,
  TAG_COLOR_ROSE: 6,
  TAG_COLOR_BLUE: 7,
  TAG_COLOR_FUCHSIA: 8,
}

@autoFields
@parentResource(Workspace.Workspace)
model Tag {
  @key tagId: Resource.Id;
  name: string;
  color: TagColor;
}

model TagListItem is Resource.List.Item<Tag>;
model TagListRequest is Resource.List.Request<Tag>;
model TagListResponse is Resource.List.Response<TagListItem>;
model TagGetRequest is Resource.Get.Request<Tag>;
model TagGetResponse is Resource.Get.Response<Tag>;
model TagCreateRequest is Resource.Create.Request<Tag>;
model TagCreateResponse is Resource.Create.Response<Tag>;
model TagUpdateRequest is Resource.Update.Request<Tag>;
model TagUpdateResponse is Resource.Update.Response<Tag>;
model TagDeleteRequest is Resource.Delete.Request<Tag>;
model TagDeleteResponse is Resource.Delete.Response<Tag>;

@Protobuf.service
interface TagService {
  TagList(...TagListRequest): TagListResponse;
  TagGet(...TagGetRequest): TagGetResponse;
  TagCreate(...TagCreateRequest): TagCreateResponse;
  TagUpdate(...TagUpdateRequest): TagUpdateResponse;
  TagDelete(...TagDeleteRequest): TagDeleteResponse;
}
