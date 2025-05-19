import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

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
@normalize
@entity
@parentResource(Workspace.Workspace)
model Tag {
  @key tagId: Resource.Id;
  name: string;
  color: TagColor;
}

@Protobuf.service
interface TagService extends Resource.CRUD<Tag> {}
