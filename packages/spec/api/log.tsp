using DevTools;

namespace Api.Log;

enum LogLevel {
  Warning,
  Error,
}

@TanStackDB.collection(#{ isReadOnly: true })
model Log {
  @primaryKey logId: Id;
  name: string;
  level: LogLevel;
  value?: Protobuf.WellKnown.Json;
}
