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

interface ExportService {
  Export(...ExportRequest): ExportResponse;
}
