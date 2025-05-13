import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

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
@normalize
@entity
@parentResource(Collection.Collection)
model Folder {
  @key folderId: Resource.Id;
  @visibility(Resource.Query.List) parentFolderId?: Resource.Id;
  name: string;
}

model FolderChange is Resource.Change<Folder>;
model FolderListItem is Resource.List.Item<Folder>;

@Protobuf.service
interface FolderService
  extends Resource.Create.Interface<Folder>,
    Resource.Update.Interface<Folder>,
    Resource.Delete.Interface<Folder> {}
