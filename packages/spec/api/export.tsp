namespace Api.Export;

model ExportRequest {
  workspaceId: Id;
  fileIds?: Id[];
}

model ExportResponse {
  name: string;
  data: bytes;
}

op Export(...ExportRequest): ExportResponse;

model ExportCurlRequest {
  workspaceId: Id;
  httpIds?: Id[];
}

model ExportCurlResponse {
  data: string;
}
op ExportCurl(...ExportCurlRequest): ExportCurlResponse;

model ExportCurlGraphQLRequest {
  workspaceId: Id;
  graphqlIds?: Id[];
}

model ExportCurlGraphQLResponse {
  data: string;
}
op ExportCurlGraphQL(...ExportCurlGraphQLRequest): ExportCurlGraphQLResponse;
