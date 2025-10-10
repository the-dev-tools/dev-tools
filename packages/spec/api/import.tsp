using DevTools;

namespace API.Import;

enum ImportMissingDataKind {
  Domain,
}

model ImportDomainData {
  enabled: boolean;
  domain: string;
  variable: string;
}

model ImportRequest {
  workspaceId: Resource.Id;
  name: string;
  data: bytes;
  textData: string;
  domainData?: ImportDomainData[];
}

model ImportResponse {
  missingData: ImportMissingDataKind;
  domains?: string[];
  flow?: Flow.FlowListItem;
}

interface ImportService {
  @DataClient.endpoint("import.js:import$")
  Import(...ImportRequest): ImportResponse;
}
