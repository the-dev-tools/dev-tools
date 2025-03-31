import "@typespec/protobuf";

import "../lib";
import "./reference.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;

@package({
  name: "log.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/log/v1;logv1",
  },
})
namespace API.Log;

@autoFields
model LogStreamResponse {
  @key logId: Resource.Id;
  value: string;
  references: Reference.Reference[];
}

@Protobuf.service
interface LogService {
  @stream(StreamMode.Out)
  LogStream(): LogStreamResponse;
}
