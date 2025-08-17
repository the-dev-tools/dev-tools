using DevTools;

namespace API.CollectionItem.Folder;

@DataClient.entity
@parent(Collection.Collection)
model Folder {
  @key folderId: Resource.Id;
  @visibility(Resource.Query.List) parentFolderId?: Resource.Id;
  name: string;
}

@instanceOf(Resource.Create.CreateResponse<Folder>)
model FolderCreateResponse is KeyOf<Folder>;

interface FolderService extends Resource.Update.Interface<Folder>, Resource.Delete.Interface<Folder> {
  @DataClient.endpoint("collection-item.js:createFolder")
  FolderCreate(...Resource.Create.CreateRequest<Folder>): FolderCreateResponse;
}
