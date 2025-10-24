namespace Api.Export;

model ExportRequest {
  workspaceId: Id;
  flowIds?: Id[];
  exampleIds?: Id[];
}

model ExportResponse {
  name: string;
  data: bytes;
}

op Export(...ExportRequest): ExportResponse;

model ExportCurlRequest {
  workspaceId: Id;
  exampleIds?: Id[];
}

model ExportCurlResponse {
  data: string;
}
op ExportCurl(...ExportCurlRequest): ExportCurlResponse;
