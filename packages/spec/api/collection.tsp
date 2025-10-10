using DevTools;

namespace API.Collection;

@DataClient.entity
@parent(Workspace.Workspace)
model Collection {
  @key collectionId: Resource.Id;
  name: string;
}

model CollectionListRequest is T.Resource.List.Request<Collection>;
model CollectionListResponse is T.Resource.List.Response<TRequest = CollectionListRequest, TItem = CollectionListItem>;
model CollectionListItem is T.Resource.List.Item<Collection>;

model CollectionGetRequest is T.Resource.Get.Request<Collection>;
model CollectionGetResponse is T.Resource.Get.Response<Collection>;

model CollectionCreateRequest is T.Resource.Create.Request<Collection>;
model CollectionCreateResponse is T.Resource.Create.Response<Collection>;

model CollectionUpdateRequest is T.Resource.Update.Request<Collection>;
model CollectionUpdateResponse is T.Resource.Update.Response<Collection>;

model CollectionDeleteRequest is T.Resource.Delete.Request<Collection>;
model CollectionDeleteResponse is T.Resource.Delete.Response<Collection>;

model CollectionMoveRequest {
  ...ParentKeyOf<Collection>;
  ...KeyOf<Collection>;
  position: Resource.MovePosition;
  targetCollectionId: Resource.Id;
}

interface CollectionService
  extends T.Resource.CRUD<
      TResource = Collection,
      // List
      TListRequest = CollectionListRequest,
      TListResponse = CollectionListResponse,
      TListItem = CollectionListItem,
      // Get
      TGetRequest = CollectionGetRequest,
      TGetResponse = CollectionGetResponse,
      // Create
      TCreateRequest = CollectionCreateRequest,
      TCreateResponse = CollectionCreateResponse,
      // Update
      TUpdateRequest = CollectionUpdateRequest,
      TUpdateResponse = CollectionUpdateResponse,
      // Delete
      TDeleteRequest = CollectionDeleteRequest,
      TDeleteResponse = CollectionDeleteResponse
    > {
  @DataClient.endpoint("collection.js:move")
  CollectionMove(...CollectionMoveRequest): {};
}
