import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.api.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/api/v1;apiv1",
  },
})
namespace API.CollectionItem.Api;

@parentResource(Collection.Collection)
model Api {
  @field(3) @key apiId: Resource.ID;
  @field(4) @visibility("list", "mutate") parentFolderId?: Resource.ID;
  @field(5) name: string;
  @field(6) method: string;
  @field(7) @visibility("get", "mutate") url: string;
}

model ApiListItem is Resource.List.Item<Api>;
model ApiGetRequest is Resource.Get.Request<Api>;
model ApiGetResponse is Resource.Get.Response<Api>;
model ApiCreateRequest is Resource.Create.Request<Api>;
model ApiCreateResponse is Resource.Create.Response<Api>;
model ApiUpdateRequest is Resource.Update.Request<Api>;
model ApiUpdateResponse is Resource.Update.Response<Api>;
model ApiDeleteRequest is Resource.Delete.Request<Api>;
model ApiDeleteResponse is Resource.Delete.Response<Api>;

model ApiDuplicateRequest is KeyOf<Api>;

@Protobuf.service
interface ApiService {
  ApiGet(...ApiGetRequest): ApiGetResponse;
  ApiCreate(...ApiCreateRequest): ApiCreateResponse;
  ApiUpdate(...ApiUpdateRequest): ApiUpdateResponse;
  ApiDelete(...ApiDeleteRequest): ApiDeleteResponse;

  ApiDuplicate(...ApiDuplicateRequest): {};
}
