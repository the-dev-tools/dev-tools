using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "environment",
  options: {
    go_package: "dev-tools-services/gen/environment",
  },
})
namespace API.Environment;

@parentResource(Workspace.Workspace)
model Environment {
  @field(2) @key environmentId: string;
  @field(3) name: string;
  @field(4) description: string;
  @field(5) @visibility("query") updated: WellKnown.Timestamp;
  @field(6) @visibility("query") isGlobal: boolean;
}

model ListEnvironmentsItem is Resource.List.Item<Environment>;
model ListEnvironmentsRequest is Resource.List.Request<Environment>;
model ListEnvironmentsResponse is Resource.List.Response<Environment, ListEnvironmentsItem>;
model GetEnvironmentRequest is Resource.Get.Request<Environment>;
model GetEnvironmentResponse is Resource.Get.Response<Environment>;
model CreateEnvironmentRequest is Resource.Create.Request<Environment>;
model CreateEnvironmentResponse is Resource.Create.Response<Environment>;
model UpdateEnvironmentRequest is Resource.Update.Request<Environment>;
model UpdateEnvironmentResponse is Resource.Update.Response<Environment>;
model DeleteEnvironmentRequest is Resource.Delete.Request<Environment>;
model DeleteEnvironmentResponse is Resource.Delete.Response<Environment>;

@Protobuf.service
interface EnvironmentService {
  List(...ListEnvironmentsRequest): ListEnvironmentsResponse;
  Get(...GetEnvironmentRequest): GetEnvironmentResponse;
  Create(...CreateEnvironmentRequest): CreateEnvironmentResponse;
  Update(...UpdateEnvironmentRequest): UpdateEnvironmentResponse;
  Delete(...DeleteEnvironmentRequest): DeleteEnvironmentResponse;
}
