import { cors as connectCors, createConnectRouter } from '@connectrpc/connect';
import { type UniversalHandler } from '@connectrpc/connect/protocol';
import {
  HttpMethod,
  HttpMiddleware,
  HttpRouter,
  HttpServer,
  HttpServerRequest,
  HttpServerResponse,
} from '@effect/platform';
import * as NodeHttpServer from '@effect/platform-node/NodeHttpServer';
import * as NodeHttpServerRequest from '@effect/platform-node/NodeHttpServerRequest';
import * as NodeRuntime from '@effect/platform-node/NodeRuntime';
import { Array, Effect, Layer, pipe, Stream } from 'effect';
import { createServer, IncomingMessage } from 'http';
import { NodeJsExecutorService } from './nodejs-executor.ts';

const connectRouter = createConnectRouter();

NodeJsExecutorService(connectRouter);

const portStr = process.env['JS_WORKER_PORT'] ?? process.env['PORT'];
const port = portStr ? parseInt(portStr, 10) : 9090;
const WorkerServerLive = NodeHttpServer.layer(createServer, { port });

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
  Layer.launch,
  NodeRuntime.runMain,
);
