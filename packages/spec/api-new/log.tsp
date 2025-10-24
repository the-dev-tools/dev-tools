using DevTools;

namespace Api.Log;

enum LogLevel {
  Warning,
  Error,
}

// TODO: read-only
@TanStackDB.collection
model Log {
  @key logId: Id;
  name: string;
  level: LogLevel;
  value?: Protobuf.WellKnown.Json;
}
