import "@typespec/protobuf";

import "./lib.tsp";
import "./resource.tsp";
import "./well-known.tsp";

using TypeSpec.Protobuf;

@package({
  name: "log.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/log/v1;logv1",
  },
})
namespace API.Log;

enum LogLevel {
  LOG_LEVEL_UNSPECIFIED: 0,
  LOG_LEVEL_WARNING: 1,
  LOG_LEVEL_ERROR: 2,
}

@autoFields
model LogStreamResponse {
  @key logId: Resource.Id;
  name: string;
  level: LogLevel;
  value?: WellKnown.JsonValue;
}

@Protobuf.service
interface LogService {
  @stream(StreamMode.Out)
  LogStream(): LogStreamResponse;
}
