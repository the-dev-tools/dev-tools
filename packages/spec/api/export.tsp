import "@typespec/rest";
import "@typespec/http";
import "./resource.tsp";

using TypeSpec.Rest;
using TypeSpec.Http;

namespace Export;

@service({
  title: "Export Service"
})
@route("/export")
interface ExportService {
  @route("export")
  export(...Export.exportRequest): Export.exportResponse;

  @route("export-curl")
  exportCurl(...Export.exportCurlRequest): Export.exportCurlResponse;
}

@autoFields
model exportRequest {
  workspaceId: Resource.Id;
  flowIds?: Resource.Id[];
  exampleIds?: Resource.Id[];
}

@autoFields
model exportResponse {
  name: string;
  data: bytes;
}

@autoFields
model exportCurlRequest {
  workspaceId: Resource.Id;
  exampleId: Resource.Id;
}

@autoFields
model exportCurlResponse {
  curlCommand: string;
  endpoint: string;
  method: string;
}