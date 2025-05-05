import "@typespec/protobuf";

import "./lib.tsp";
import "./well-known.tsp";

using TypeSpec.Protobuf;

@package({
  name: "nodejs_executor.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/nodejs_executor/v1;nodejs_executorv1",
  },
})
namespace API.NodeJSExecutor;

@autoFields
model ExecuteNodeJSRequest {
  context: WellKnown.JsonValue;
  code: string;
}

@autoFields
model ExecuteNodeJSResponse {
  result: WellKnown.JsonValue;
}

@Protobuf.service
interface NodeJSExecutorService {
  ExecuteNodeJS(...ExecuteNodeJSRequest): ExecuteNodeJSResponse;
}
