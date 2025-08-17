using DevTools;

namespace API.CollectionItem.Item;

enum ItemKind {
  Folder,
  Endpoint,
}

@parent(Collection.Collection)
model CollectionItem {
  kind: ItemKind;
  folder?: Resource.List.ListItem<Folder.Folder>;
  endpoint?: Resource.List.ListItem<Endpoint.Endpoint>;
  example?: Resource.List.ListItem<Example.Example>;
}

model CollectionItemListRequest is Resource.List.ListRequest<CollectionItem> {
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
