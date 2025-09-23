import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./resource.tsp";

using TypeSpec.Protobuf;

@package({
  name: "export.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/export/v1;exportv1",
  },
})
namespace API.Export;

@autoFields
model ExportRequest {
  workspaceId: Resource.Id;
  flowIds?: Resource.Id[];
  exampleIds?: Resource.Id[];
}

@autoFields
model ExportResponse {
  name: string;
  data: bytes;
}

@autoFields
model ExportCurlRequest {
  workspaceId: Resource.Id;
  exampleIds?: Resource.Id[];
}

@autoFields
model ExportCurlResponse {
  data: string;
}

@Protobuf.service
interface ExportService {
  Export(...ExportRequest): ExportResponse;
  ExportCurl(...ExportCurlRequest): ExportCurlResponse;
}
