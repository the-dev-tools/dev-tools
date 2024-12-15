import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/v1;collectionv1",
  },
})
namespace API.Collection;

@autoFields
@parentResource(Workspace.Workspace)
model Collection {
  @key collectionId: Resource.Id;
  name: string;
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

@autoFields
model CollectionImportPostmanRequest {
  ...Resource.Create.Request<Collection>;
  data: bytes;
}
model CollectionImportPostmanResponse is Resource.Create.Response<Collection>;

@autoFields
model CollectionImportHarRequest {
  ...Resource.Create.Request<Collection>;
  data: bytes;
}
model CollectionImportHarResponse is Resource.Create.Response<Collection>;

@Protobuf.service
interface CollectionService {
  CollectionList(...CollectionListRequest): CollectionListResponse;
  CollectionGet(...CollectionGetRequest): CollectionGetResponse;
  CollectionCreate(...CollectionCreateRequest): CollectionCreateResponse;
  CollectionUpdate(...CollectionUpdateRequest): CollectionUpdateResponse;
  CollectionDelete(...CollectionDeleteRequest): CollectionDeleteResponse;

  CollectionImportPostman(...CollectionImportPostmanRequest): CollectionImportPostmanResponse;
  CollectionImportHar(...CollectionImportHarRequest): CollectionImportHarResponse;
}
