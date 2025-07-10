import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/v1;collectionv1",
  },
})
namespace API.Collection;

@autoFields
@normalize
@entity
@parentResource(Workspace.Workspace)
model Collection {
  @key collectionId: Resource.Id;
  name: string;
}

model CollectionListItem is Resource.List.Item<Collection>;

@autoFields
model CollectionMoveRequest {
  ...ParentKeyOf<Collection>;
  ...KeyOf<Collection>;
  position: Resource.MovePosition;
  targetCollectionId: Resource.Id;
}

model CollectionMoveResponse {}

@Protobuf.service
interface CollectionService extends Resource.CRUD<Collection, TListItem = CollectionListItem> {
  @endpoint("collection.js:move")
  CollectionMove(...CollectionMoveRequest): CollectionMoveResponse;
}
