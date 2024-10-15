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
    go_package: "dev-tools-services/gen/collection/item/api/v1;apiv1",
  },
})
namespace API.CollectionItem.Api;

@parentResource(Collection.Collection)
model Api {
  @field(3) @key apiId: Resource.ID;
  @field(4) name: string;
  @field(5) method: string;
  @field(6) @visibility("get") url: string;
}

model ListApiItem is Resource.List.Item<Api>;
model GetApiRequest is Resource.Get.Request<Api>;
model GetApiResponse is Resource.Get.Response<Api>;
model CreateApiRequest is Resource.Create.Request<Api>;
model CreateApiResponse is Resource.Create.Response<Api>;
model UpdateApiRequest is Resource.Update.Request<Api>;
model UpdateApiResponse is Resource.Update.Response<Api>;
model DeleteApiRequest is Resource.Delete.Request<Api>;
model DeleteApiResponse is Resource.Delete.Response<Api>;

model DupliacteApiRequest is KeyOf<Api>;

@Protobuf.service
interface CollectionItemApiService {
  GetApi(...GetApiRequest): GetApiResponse;
  CreateApi(...CreateApiRequest): CreateApiResponse;
  UpdateApi(...UpdateApiRequest): UpdateApiResponse;
  DeleteApi(...DeleteApiRequest): DeleteApiResponse;

  DupliacteApi(...DupliacteApiRequest): {};
}
