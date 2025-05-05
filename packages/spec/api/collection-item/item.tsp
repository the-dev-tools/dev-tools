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
  @normalKey folderId?: Resource.Id;
}

model CollectionItemListResponse is Resource.List.Response<{}, CollectionItem> {
  ...CollectionItemListRequest;
}

@autoFields
model CollectionItemMoveRequest {
  ...ParentKeyOf<CollectionItem>;
  itemId: Resource.Id;
  kind: ItemKind;
  parentFolderId?: Resource.Id;
  previousItemId: Resource.Id;
  nextItemId: Resource.Id;
}

@Protobuf.service
interface CollectionItemService {
  CollectionItemList(...CollectionItemListRequest): CollectionItemListResponse;
  CollectionItemMove(...CollectionItemMoveRequest): {};
}
