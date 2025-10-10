using DevTools;

namespace API.Log;

enum LogLevel {
  Warning,
  Error,
}

model LogStreamResponse {
  @key logId: Resource.Id;
  name: string;
  level: LogLevel;
  value?: Protobuf.WellKnown.Json;
}

interface LogService {
  @Protobuf.stream(Protobuf.StreamMode.Out)
  LogStream(): LogStreamResponse;
}
