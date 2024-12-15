import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../collection.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.folder.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1;folderv1",
  },
})
namespace API.CollectionItem.Folder;

@autoFields
@parentResource(Collection.Collection)
model Folder {
  @key folderId: Resource.Id;
  @visibility("list", "mutate") parentFolderId?: Resource.Id;
  name: string;
}

model FolderListItem is Resource.List.Item<Folder>;
model FolderCreateRequest is Resource.Create.Request<Folder>;
model FolderCreateResponse is Resource.Create.Response<Folder>;
model FolderUpdateRequest is Resource.Update.Request<Folder>;
model FolderUpdateResponse is Resource.Update.Response<Folder>;
model FolderDeleteRequest is Resource.Delete.Request<Folder> {
  @autoField parentFolderId?: Resource.Id;
}
model FolderDeleteResponse is Resource.Delete.Response<Folder>;

@Protobuf.service
interface FolderService {
  FolderCreate(...FolderCreateRequest): FolderCreateResponse;
  FolderUpdate(...FolderUpdateRequest): FolderUpdateResponse;
  FolderDelete(...FolderDeleteRequest): FolderDeleteResponse;
}
