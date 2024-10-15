import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.folder.v1",
  options: {
    go_package: "dev-tools-services/gen/collection/item/folder/v1;folderv1",
  },
})
namespace API.CollectionItem.Folder;

@parentResource(Collection.Collection)
model Folder {
  @field(3) @key folderId: Resource.ID;
  @field(4) name: string;
}

model CreateFolderRequest is Resource.Create.Request<Folder>;
model CreateFolderResponse is Resource.Create.Response<Folder>;
model UpdateFolderRequest is Resource.Update.Request<Folder>;
model UpdateFolderResponse is Resource.Update.Response<Folder>;
model DeleteFolderRequest is Resource.Delete.Request<Folder>;
model DeleteFolderResponse is Resource.Delete.Response<Folder>;

@Protobuf.service
interface FolderService {
  CreateFolder(...CreateFolderRequest): CreateFolderResponse;
  UpdateFolder(...UpdateFolderRequest): UpdateFolderResponse;
  DeleteFolder(...DeleteFolderRequest): DeleteFolderResponse;
}
