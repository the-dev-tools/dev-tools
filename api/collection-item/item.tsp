import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";
import "./api.tsp";
import "./example.tsp";
import "./folder.tsp";

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
}

@parentResource(Collection.Collection)
model CollectionItem {
  @field(3) kind: ItemKind;
  @field(4) folder?: Folder.ListFoldersItem;
  @field(5) api?: Api.ListApisItem;
  @field(6) example?: Example.ListExamplesItem;
}

model ListItemsRequest is Resource.List.Request<CollectionItem> {
  @field(3) folderId?: Resource.ID;
}

model ListItemsResponse is Resource.List.Response<CollectionItem>;

model MoveItemRequest {
  ...ParentKeyOf<CollectionItem>;
  @field(3) itemId: Resource.ID;
  @field(4) kind: ItemKind;
  @field(5) parentFolderId?: Resource.ID;
  @field(6) previousItemId: Resource.ID;
  @field(7) nextItemId: Resource.ID;
}

@Protobuf.service
interface CollectionItemService {
  ListItems(...ListItemsRequest): ListItemsResponse;
  MoveItem(...MoveItemRequest): {};
}
