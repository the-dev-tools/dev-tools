using DevTools;

namespace Api.WebSocket;

@TanStackDB.collection
model WebSocket {
  @primaryKey websocketId: Id;
  name: string;
  url: string;
  lastRunAt?: Protobuf.WellKnown.Timestamp;
}

@TanStackDB.collection
model WebSocketHeader {
  @primaryKey websocketHeaderId: Id;
  ...CommonTableFields<WebSocket>;
}

model WebSocketRunRequest {
  websocketId: Id;
}

op WebSocketRun(...WebSocketRunRequest): {};
