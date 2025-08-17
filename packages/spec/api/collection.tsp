using DevTools;

namespace API.Collection;

@DataClient.entity
@parent(Workspace.Workspace)
model Collection {
  @key collectionId: Resource.Id;
  name: string;
}

model CollectionMoveRequest {
  ...ParentKeyOf<Collection>;
  ...KeyOf<Collection>;
  position: Resource.MovePosition;
  targetCollectionId: Resource.Id;
}

model CollectionMoveResponse {}

interface CollectionService extends Resource.CRUD<Collection> {
  @DataClient.endpoint("collection.js:move")
  CollectionMove(...CollectionMoveRequest): CollectionMoveResponse;
}
