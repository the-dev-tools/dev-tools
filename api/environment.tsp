import "@typespec/protobuf";
import "@typespec/rest";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "environment.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/environment/v1;environmentv1",
  },
})
namespace API.Environment;

@parentResource(Workspace.Workspace)
model Environment {
  @field(2) @key environmentId: Resource.ID;
  @field(3) name: string;
  @field(4) description: string;
  @field(5) @visibility("query") updated: WellKnown.Timestamp;
  @field(6) @visibility("query") isGlobal: boolean;
}

model EnvironmentListItem is Resource.List.Item<Environment>;
model EnvironmentListRequest is Resource.List.Request<Environment>;
model EnvironmentListResponse is Resource.List.Response<EnvironmentListItem>;
model EnvironmentGetRequest is Resource.Get.Request<Environment>;
model EnvironmentGetResponse is Resource.Get.Response<Environment>;
model EnvironmentCreateRequest is Resource.Create.Request<Environment>;
model EnvironmentCreateResponse is Resource.Create.Response<Environment>;
model EnvironmentUpdateRequest is Resource.Update.Request<Environment>;
model EnvironmentUpdateResponse is Resource.Update.Response<Environment>;
model EnvironmentDeleteRequest is Resource.Delete.Request<Environment>;
model EnvironmentDeleteResponse is Resource.Delete.Response<Environment>;

@Protobuf.service
interface EnvironmentService {
  EnvironmentList(...EnvironmentListRequest): EnvironmentListResponse;
  EnvironmentGet(...EnvironmentGetRequest): EnvironmentGetResponse;
  EnvironmentCreate(...EnvironmentCreateRequest): EnvironmentCreateResponse;
  EnvironmentUpdate(...EnvironmentUpdateRequest): EnvironmentUpdateResponse;
  EnvironmentDelete(...EnvironmentDeleteRequest): EnvironmentDeleteResponse;
}
