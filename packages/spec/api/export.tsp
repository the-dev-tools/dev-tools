namespace API.Export;

model ExportRequest {
  workspaceId: Resource.Id;
  flowIds?: Resource.Id[];
  exampleIds?: Resource.Id[];
}

model ExportResponse {
  name: string;
  data: bytes;
}

model ExportCurlRequest {
  workspaceId: Resource.Id;
  exampleIds?: Resource.Id[];
}

model ExportCurlResponse {
  data: string;
}

interface ExportService {
  Export(...ExportRequest): ExportResponse;
  ExportCurl(...ExportCurlRequest): ExportCurlResponse;
}
