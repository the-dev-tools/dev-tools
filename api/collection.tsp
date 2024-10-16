import "@typespec/protobuf";
import "@typespec/rest";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/v1;collectionv1",
  },
})
namespace API.Collection;

@parentResource(Workspace.Workspace)
model Collection {
  @field(2) @key collectionId: Resource.ID;
  @field(3) name: string;
}

model CollectionListItem is Resource.List.Item<Collection>;
model CollectionListRequest is Resource.List.Request<Collection>;
model CollectionListResponse is Resource.List.Response<CollectionListItem>;
model CollectionGetRequest is Resource.Get.Request<Collection>;
model CollectionGetResponse is Resource.Get.Response<Collection>;
model CollectionCreateRequest is Resource.Create.Request<Collection>;
model CollectionCreateResponse is Resource.Create.Response<Collection>;
model CollectionUpdateRequest is Resource.Update.Request<Collection>;
model CollectionUpdateResponse is Resource.Update.Response<Collection>;
model CollectionDeleteRequest is Resource.Delete.Request<Collection>;
model CollectionDeleteResponse is Resource.Delete.Response<Collection>;

model CollectionImportPostmanRequest {
  ...Resource.Create.Request<Collection>;
  @field(4) data: bytes;
}
model CollectionImportPostmanResponse is Resource.Create.Response<Collection>;

@Protobuf.service
interface CollectionService {
  CollectionList(...CollectionListRequest): CollectionListResponse;
  CollectionGet(...CollectionGetRequest): CollectionGetResponse;
  CollectionCreate(...CollectionCreateRequest): CollectionCreateResponse;
  CollectionUpdate(...CollectionUpdateRequest): CollectionUpdateResponse;
  CollectionDelete(...CollectionDeleteRequest): CollectionDeleteResponse;

  CollectionImportPostman(...CollectionImportPostmanRequest): CollectionImportPostmanResponse;
}
