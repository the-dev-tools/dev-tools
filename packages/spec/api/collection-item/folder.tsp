using DevTools;

namespace API.Collection.Item.Folder;

@DataClient.entity
@parent(Collection)
model Folder {
  @key folderId: Resource.Id;
  @visibility(T.Resource.Query.List) parentFolderId?: Resource.Id;
  name: string;
}

model FolderListItem is T.Resource.List.Item<Folder>;

model FolderCreateRequest is T.Resource.Create.Request<Folder>;
model FolderCreateResponse is KeyOf<Folder>;

model FolderUpdateRequest is T.Resource.Update.Request<Folder>;
model FolderUpdateResponse is T.Resource.Update.Response<Folder>;

model FolderDeleteRequest is T.Resource.Delete.Request<Folder>;
model FolderDeleteResponse is T.Resource.Delete.Response<Folder>;

interface FolderService
  extends T.Resource.Update.Interface<
      TResource = Folder,
      TRequest = FolderUpdateRequest,
      TResponse = FolderUpdateResponse
    >,
    T.Resource.Delete.Interface<TResource = Folder, TRequest = FolderDeleteRequest, TResponse = FolderDeleteResponse> {
  @DataClient.endpoint("collection-item.js:createFolder")
  FolderCreate(...FolderCreateRequest): FolderCreateResponse;
}
