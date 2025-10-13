using DevTools;

namespace API.NodeJsExecutor;

model NodeJsExecutorRunRequest {
  context: Protobuf.WellKnown.Json;
  code: string;
}

model NodeJsExecutorRunResponse {
  result: Protobuf.WellKnown.Json;
}

interface NodeJsExecutorService {
  NodeJsExecutorRun(...NodeJsExecutorRunRequest): NodeJsExecutorRunResponse;
}
