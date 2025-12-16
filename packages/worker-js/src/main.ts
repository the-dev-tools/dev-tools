import { cors as connectCors, createConnectRouter } from '@connectrpc/connect';
import { type UniversalHandler } from '@connectrpc/connect/protocol';
import {
  FileSystem,
  HttpMethod,
  HttpMiddleware,
  HttpRouter,
  HttpServer,
  HttpServerRequest,
  HttpServerResponse,
  Path,
} from '@effect/platform';
import * as NodeContext from '@effect/platform-node/NodeContext';
import * as NodeHttpServer from '@effect/platform-node/NodeHttpServer';
import * as NodeHttpServerRequest from '@effect/platform-node/NodeHttpServerRequest';
import * as NodeRuntime from '@effect/platform-node/NodeRuntime';
import { Array, Config, Effect, Layer, pipe, Stream } from 'effect';
import { createServer, IncomingMessage } from 'http';
import os from 'node:os';
import { NodeJsExecutorService } from './nodejs-executor.ts';

const connectRouter = createConnectRouter();

NodeJsExecutorService(connectRouter);

// Environment variables:
//   - WORKER_MODE: "uds" (default) or "tcp"
//   - WORKER_SOCKET_PATH: custom socket path (uds mode)
//   - WORKER_PORT: port number (tcp mode, defaults to 9090)

const WorkerServerUdsLive = Effect.gen(function* () {
  const path = yield* Path.Path;
  const fs = yield* FileSystem.FileSystem;

  const directory = path.join(os.tmpdir(), 'the-dev-tools');

  yield* fs.makeDirectory(directory, { recursive: true });

  const socket = yield* pipe(
    Config.string('WORKER_SOCKET_PATH'),
    Config.withDefault(path.join(directory, 'worker-js.socket')),
  );

  // Try deleting a possibly hanging socket before acquiring a new one
  yield* fs.remove(socket, { force: true });

  return yield* Effect.acquireRelease(
    // Acquire socket & create server
    pipe(NodeHttpServer.layer(createServer, { path: socket }), Layer.build),
    // Release socket
    () => pipe(fs.remove(socket, { force: true }), Effect.orDie),
  );
});

const WorkerServerTcpLive = Effect.gen(function* () {
  const port = yield* pipe(Config.port('WORKER_PORT'), Config.withDefault(9090));
  return yield* pipe(NodeHttpServer.layer(createServer, { port }), Layer.build);
});

const WorkerServerLive = Effect.gen(function* () {
  const mode = yield* pipe('WORKER_MODE', Config.literal('uds', 'tcp'), Config.withDefault('uds'));
  if (mode === 'tcp') return yield* WorkerServerTcpLive;
  return yield* WorkerServerUdsLive;
}).pipe(Layer.effectContext);

async function* asyncIterableFromNodeServerRequest(request: IncomingMessage) {
  for await (const chunk of request) {
    yield chunk;
  }
}

const toEffectHandler = Effect.fn(function* (handler: UniversalHandler) {
  const request = yield* HttpServerRequest.HttpServerRequest;
  const requestRaw = NodeHttpServerRequest.toIncomingMessage(request);

  const response = yield* Effect.tryPromise((signal) =>
    handler({
      body: asyncIterableFromNodeServerRequest(requestRaw),
      header: new Headers(request.headers),
      httpVersion: requestRaw.httpVersion,
      method: request.method,
      signal,
      url: new URL(request.url, `http://${request.headers['host']}`).toString(),
    }),
  );

  const body = yield* pipe(
    Effect.fromNullable(response.body),
    Effect.map((_) => Stream.fromAsyncIterable(_, (e) => new Error(String(e)))),
  );

  return yield* HttpServerResponse.stream(body, {
    headers: response.header,
    status: response.status,
  });
}, Effect.onError(Effect.logError));

const routes = Array.flatMap(connectRouter.handlers, (handler) =>
  handler.allowedMethods.map((method) =>
    HttpRouter.makeRoute(
      method as HttpMethod.HttpMethod,
      handler.requestPath as HttpRouter.PathInput,
      toEffectHandler(handler),
    ),
  ),
);

pipe(
  HttpRouter.fromIterable(routes),
  HttpMiddleware.cors(connectCors),
  HttpServer.serve(),
  HttpServer.withLogAddress,
  Layer.provide(WorkerServerLive),
  Layer.provide(NodeContext.layer),
  Layer.provide(Layer.scope),
  Layer.launch,
  NodeRuntime.runMain,
);
