import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.folder.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/folder/v1;folderv1",
  },
})
namespace API.CollectionItem.Folder;

@parentResource(Collection.Collection)
model Folder {
  @field(3) @key folderId: Resource.ID;
  @field(4) @visibility("list", "mutate") parentFolderId?: Resource.ID;
  @field(5) name: string;
}

model FolderListItem is Resource.List.Item<Folder>;
model FolderCreateRequest is Resource.Create.Request<Folder>;
model FolderCreateResponse is Resource.Create.Response<Folder>;
model FolderUpdateRequest is Resource.Update.Request<Folder>;
model FolderUpdateResponse is Resource.Update.Response<Folder>;
model FolderDeleteRequest is Resource.Delete.Request<Folder>;
model FolderDeleteResponse is Resource.Delete.Response<Folder>;

@Protobuf.service
interface FolderService {
  FolderCreate(...FolderCreateRequest): FolderCreateResponse;
  FolderUpdate(...FolderUpdateRequest): FolderUpdateResponse;
  FolderDelete(...FolderDeleteRequest): FolderDeleteResponse;
}
