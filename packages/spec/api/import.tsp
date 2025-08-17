using DevTools;

namespace API.Import;

enum ImportKind {
  Filter,
}

model ImportRequest {
  workspaceId: Resource.Id;
  name: string;
  kind: ImportKind;
  data: bytes;
  textData: string;
  filter?: string[];
}

model ImportResponse {
  kind: ImportKind;
  filter?: string[];
  flow?: Resource.List.ListItem<Flow.Flow>;
}

interface ImportService {
  @DataClient.endpoint("import.js:import$")
  Import(...ImportRequest): ImportResponse;
}
