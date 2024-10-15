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

model ListCollectionsItem is Resource.List.Item<Collection>;
model ListCollectionsRequest is Resource.List.Request<Collection>;
model ListCollectionsResponse is Resource.List.Response<ListCollectionsItem>;
model GetCollectionRequest is Resource.Get.Request<Collection>;
model GetCollectionResponse is Resource.Get.Response<Collection>;
model CreateCollectionRequest is Resource.Create.Request<Collection>;
model CreateCollectionResponse is Resource.Create.Response<Collection>;
model UpdateCollectionRequest is Resource.Update.Request<Collection>;
model UpdateCollectionResponse is Resource.Update.Response<Collection>;
model DeleteCollectionRequest is Resource.Delete.Request<Collection>;
model DeleteCollectionResponse is Resource.Delete.Response<Collection>;

model ImportPostmanRequest {
  ...Resource.Create.Request<Collection>;
  @field(4) data: bytes;
}
model ImportPostmanResponse is Resource.Create.Response<Collection>;

@Protobuf.service
interface CollectionService {
  ListCollections(...ListCollectionsRequest): ListCollectionsResponse;
  GetCollection(...GetCollectionRequest): GetCollectionResponse;
  CreateCollection(...CreateCollectionRequest): CreateCollectionResponse;
  UpdateCollection(...UpdateCollectionRequest): UpdateCollectionResponse;
  DeleteCollection(...DeleteCollectionRequest): DeleteCollectionResponse;

  ImportPostman(...ImportPostmanRequest): ImportPostmanResponse;
}
