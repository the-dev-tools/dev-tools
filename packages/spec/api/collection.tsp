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
@normalize
@parentResource(Workspace.Workspace)
model Collection {
  @key collectionId: Resource.Id;
  name: string;
}

model CollectionChange is Resource.Change<Collection>;
model CollectionListItem is Resource.List.Item<Collection>;
model CollectionListRequest is Resource.List.Request<Collection>;
model CollectionListResponse is Resource.List.Response<Collection, CollectionListItem>;
model CollectionGetRequest is Resource.Get.Request<Collection>;
model CollectionGetResponse is Resource.Get.Response<Collection>;
model CollectionCreateRequest is Resource.Create.Request<Collection>;
model CollectionCreateResponse is Resource.Create.Response<Collection, CollectionListResponse>;
model CollectionUpdateRequest is Resource.Update.Request<Collection>;
model CollectionUpdateResponse is Resource.Update.Response<Collection>;
model CollectionDeleteRequest is Resource.Delete.Request<Collection>;
model CollectionDeleteResponse is Resource.Delete.Response<Collection>;

@Protobuf.service
interface CollectionService {
  CollectionList(...CollectionListRequest): CollectionListResponse;
  CollectionGet(...CollectionGetRequest): CollectionGetResponse;
  CollectionCreate(...CollectionCreateRequest): CollectionCreateResponse;
  CollectionUpdate(...CollectionUpdateRequest): CollectionUpdateResponse;
  CollectionDelete(...CollectionDeleteRequest): CollectionDeleteResponse;
}
