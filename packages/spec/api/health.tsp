import "@typespec/protobuf";

using TypeSpec.Protobuf;

@package({
  name: "health.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/health/v1;healthv1",
  },
})
namespace API.Health;

model HealthCheckRequest {}
model HealthCheckResponse {}

@Protobuf.service
interface HealthService {
  HealthCheck(...HealthCheckRequest): HealthCheckResponse;
}
