import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./resource.tsp";
import "./workspace.tsp";

using TypeSpec.Protobuf;

@package({
  name: "import.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/import/v1;importv1",
  },
})
namespace API.Import;

enum ImportMissingDataKind {
  IMPORT_MISSING_DATA_KIND_UNSPECIFIED: 0,
  IMPORT_MISSING_DATA_KIND_DOMAIN: 1,
}

@autoFields
model ImportDomainData {
  enabled: boolean;
  domain: string;
  variable: string;
}

@autoFields
model ImportRequest {
  workspaceId: Resource.Id;
  name: string;
  data: bytes;
  textData: string;
  domainData?: ImportDomainData[];
}

@autoFields
model ImportResponse {
  missingData: ImportMissingDataKind;
  domains?: string[];
  flow?: Flow.FlowListItem;
}

@Protobuf.service
interface ImportService {
  @endpoint("import.js:import$")
  Import(...ImportRequest): ImportResponse;
}
