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

@Protobuf.service
interface CollectionService extends Resource.CRUD<Collection, TListItem = CollectionListItem> {}
