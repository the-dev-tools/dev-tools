import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../collection.tsp";
import "../resource.tsp";
import "./endpoint.tsp";
import "./example.tsp";
import "./folder.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/v1;itemv1",
  },
})
namespace API.CollectionItem.Item;

enum ItemKind {
  ITEM_KIND_UNSPECIFIED: 0,
  ITEM_KIND_FOLDER: 1,
  ITEM_KIND_ENDPOINT: 2,
}

@autoFields
@parentResource(Collection.Collection)
model CollectionItem {
  kind: ItemKind;
  folder?: Folder.FolderListItem;
  endpoint?: Endpoint.EndpointListItem;
  example?: Example.ExampleListItem;
}

@autoFields
model CollectionItemListRequest is Resource.List.Request<CollectionItem> {
  @normalKey parentFolderId?: Resource.Id;
}

@autoFields
model CollectionItemListResponse {
  items: CollectionItem[];
}

@autoFields
model CollectionItemMoveRequest {
  kind: ItemKind;
  itemId: Resource.Id;
  collectionId: Resource.Id;
  parentFolderId?: Resource.Id;
  targetCollectionId?: Resource.Id;
  targetParentFolderId?: Resource.Id;
  targetKind?: ItemKind;
  targetItemId?: Resource.Id;
  position?: Resource.MovePosition;
}

model CollectionItemMoveResponse {}

@Protobuf.service
interface CollectionItemService {
  @endpoint("collection-item.js:list")
  CollectionItemList(...CollectionItemListRequest): CollectionItemListResponse;
  @endpoint("collection-item.js:move")
  CollectionItemMove(...CollectionItemMoveRequest): CollectionItemMoveResponse;
}
