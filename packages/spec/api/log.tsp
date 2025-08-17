using DevTools;

namespace API.Log;

enum LogLevel {
  Warning,
  Error,
}

model LogStreamResponse {
  @key logId: Resource.Id;
  value: string;
  level: LogLevel;
  references: Reference.ReferenceTreeItem[];
}

interface LogService {
  @Protobuf.stream(Protobuf.StreamMode.Out)
  LogStream(): LogStreamResponse;
}
