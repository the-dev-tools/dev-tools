using DevTools;

namespace API.NodeJsExecutor;

model ExecuteNodeJsRequest {
  context: Protobuf.WellKnown.Json;
  code: string;
}

model ExecuteNodeJsResponse {
  result: Protobuf.WellKnown.Json;
}

interface NodeJsExecutorService {
  ExecuteNodeJS(...ExecuteNodeJsRequest): ExecuteNodeJsResponse;
}
