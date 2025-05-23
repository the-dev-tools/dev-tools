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

enum ImportKind {
  IMPORT_KIND_UNSPECIFIED: 0,
  IMPORT_KIND_FILTER: 1,
}

@autoFields
model ImportRequest {
  workspaceId: Resource.Id;
  name: string;
  kind: ImportKind;
  data: bytes;
  textData: string;
  filter?: string[];
}

@autoFields
model ImportResponse {
  kind: ImportKind;
  filter?: string[];
  collection?: Collection.CollectionListItem;
  flow?: Flow.FlowListItem;
}

@Protobuf.service
interface ImportService {
  @endpoint("import.js:import$")
  Import(...ImportRequest): ImportResponse;
}
