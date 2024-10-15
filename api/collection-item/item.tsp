import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";
import "./api.tsp";
import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/v1;itemv1",
  },
})
namespace API.CollectionItem.Item;

enum ItemKind {
  ITEM_KIND_UNSPECIFIED: 0,
  ITEM_KIND_FOLDER: 1,
  ITEM_KIND_API: 2,
  ITEM_KIND_EXAMPLE: 3,
}

@parentResource(Collection.Collection)
model CollectionItem {
  @field(3) @key itemId: Resource.ID;
  @field(4) parentItemId?: string;
  @field(5) kind: ItemKind;
  @field(6) name: string;
  @field(7) api?: Api.ListApiItem;
  @field(8) example?: Example.ListExampleItem;
}

model ListItemsRequest is Resource.List.Request<CollectionItem> {
  ...PickProperties<CollectionItem, "parentItemId">;
}

model ListItemsResponse is Resource.List.Response<CollectionItem>;

model MoveItemRequest {
  ...KeyOf<CollectionItem>;
  ...PickProperties<CollectionItem, "parentItemId">;
  @field(5) previousItemId: string;
  @field(6) nextItemId: string;
}

@Protobuf.service
interface CollectionItemService {
  ListItems(...ListItemsRequest): ListItemsResponse;
  MoveItem(...MoveItemRequest): {};
}
