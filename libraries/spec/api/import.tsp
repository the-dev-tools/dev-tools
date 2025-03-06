import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

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
  kind: ImportKind;
  data: bytes;
  filter?: string[];
}

@autoFields
model ImportResponse {
  kind: ImportKind;
  filter?: string[];
  ...API.Change.Changes;
}

@Protobuf.service
interface ImportService {
  Import(...ImportRequest): ImportResponse;
}
