import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";
import "./api.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.v1",
  options: {
    go_package: "dev-tools-services/gen/collection/item/v1;collectionv1",
  },
})
namespace API.CollectionItem.Item;

enum ItemKind {
  ITEM_KIND_UNSPECIFIED: 0,
  ITEM_KIND_FOLDER: 1,
  ITEM_KIND_API: 2,
  ITEM_KIND_EXAMPLE: 3,
}

model ApiItem is Resource.List.Request<Api.Api>;

@parentResource(Collection.Collection)
model Item {
  @field(3) @key itemId: string;
  @field(4) parentItemId?: string;
  @field(5) kind: ItemKind;
  @field(6) name: string;
  @field(7) api?: ApiItem;
}

model ListItemsRequest is Resource.List.Request<Item> {
  ...PickProperties<Item, "parentItemId">;
}

model ListItemsResponse is Resource.List.Response<Item, Item>;

model MoveItemRequest {
  ...KeysOf<Item>;
  ...PickProperties<Item, "parentItemId">;
  @field(5) previousItemId: string;
  @field(6) nextItemId: string;
}

@Protobuf.service
interface CollectionItemService {
  ListItems(...ListItemsRequest): ListItemsResponse;
  MoveItem(...MoveItemRequest): void;
}
