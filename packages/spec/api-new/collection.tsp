using DevTools;

namespace Api.Collection;

model Collection {
  collectionId: Id;
  name: string;
}

model CollectionListItem {
  collectionId: Id;
  name: string;
}

model CollectionListRequest {
  workspaceId: Id;
}

model CollectionListResponse {
  collections: CollectionListItem[];
}

model CollectionGetRequest {
  collectionId: Id;
}

model CollectionGetResponse {
  collection: Collection;
}

model CollectionCreateRequest {
  workspaceId: Id;
  name: string;
}

model CollectionCreateResponse {
  collection: Collection;
}

model CollectionUpdateRequest {
  collectionId: Id;
  name: string;
}

model CollectionUpdateResponse {
  collection: Collection;
}

model CollectionDeleteRequest {
  collectionId: Id;
}

model CollectionDeleteResponse {}