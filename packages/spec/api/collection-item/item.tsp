using DevTools;

namespace API.Collection.Item;

enum ItemKind {
  Folder,
  Endpoint,
}

@parent(Collection)
model CollectionItem {
  kind: ItemKind;
  folder?: Folder.FolderListItem;
  endpoint?: Endpoint.EndpointListItem;
  example?: Example.ExampleListItem;
}

model CollectionItemListRequest is T.Resource.List.Request<CollectionItem> {
  @normalKey parentFolderId?: Resource.Id;
}

model CollectionItemListResponse {
  items: CollectionItem[];
}

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

interface CollectionItemService {
  @DataClient.endpoint("collection-item.js:list")
  CollectionItemList(...CollectionItemListRequest): CollectionItemListResponse;
  @DataClient.endpoint("collection-item.js:move")
  CollectionItemMove(...CollectionItemMoveRequest): CollectionItemMoveResponse;
}
