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

enum ExportFormat {
  EXPORT_FORMAT_UNSPECIFIED: 0,
  EXPORT_FORMAT_BINARY: 1,
  EXPORT_FORMAT_CURL: 2,
}

@autoFields
model ExportRequest {
  workspaceId: Resource.Id;
  flowIds?: Resource.Id[];
  exampleIds?: Resource.Id[];
  format?: ExportFormat;
}

@autoFields
model ExportExampleRequest {
  workspaceId: Resource.Id;
  exampleId: Resource.Id;
  format?: ExportFormat;
}

@autoFields
model ExportResponse {
  name: string;
  data?: bytes;
  textData?: string;
}

@autoFields
model ExportExampleResponse {
  name: string;
  data?: bytes;
  textData?: string;
}

@Protobuf.service
interface ExportService {
  Export(...ExportRequest): ExportResponse;
  ExportExample(...ExportExampleRequest): ExportExampleResponse;
}
