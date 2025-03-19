import { createServer } from 'http';
import { JsonValue } from '@bufbuild/protobuf';
import { cors as connectCors, createConnectRouter } from '@connectrpc/connect';
import { UniversalHandler } from '@connectrpc/connect/protocol';
import {
  HttpMethod,
  HttpMiddleware,
  HttpRouter,
  HttpServer,
  HttpServerRequest,
  HttpServerResponse,
} from '@effect/platform';
import { NodeHttpServer, NodeHttpServerRequest } from '@effect/platform-node';
import { Array, Chunk, Console, Effect, Layer, pipe, Schema, Stream } from 'effect';

const connectRouter = createConnectRouter();

const WorkerServerLive = NodeHttpServer.layer(createServer, { port: 9090 });

const toEffectHandler = Effect.fn(function* (handler: UniversalHandler) {
  const request = yield* HttpServerRequest.HttpServerRequest;
  const requestBody = yield* HttpServerRequest.schemaBodyJson(Schema.Unknown);
  const requestRaw = NodeHttpServerRequest.toIncomingMessage(request);

  const response = yield* Effect.tryPromise((signal) =>
    handler({
      httpVersion: requestRaw.httpVersion,
      url: new URL(request.url, `http://${request.headers['host']}`).toString(),
      method: request.method,
      header: new Headers(request.headers),
      body: requestBody as JsonValue,
      signal,
    }),
  );

  // Streaming responses is not supported with this implementation
  const responseBody = yield* pipe(
    Effect.fromNullable(response.body),
    Effect.map((_) => Stream.fromAsyncIterable(_, (e) => new Error(String(e)))),
    Effect.flatMap(Stream.runCollect),
    Effect.map(Chunk.toReadonlyArray),
    Effect.flatMap(Array.head),
  );

  return yield* HttpServerResponse.uint8Array(responseBody, {
    status: response.status,
    headers: response.header,
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

export const worker = pipe(
  HttpRouter.fromIterable(routes),
  HttpMiddleware.cors(connectCors),
  HttpServer.serve(),
  HttpServer.withLogAddress,
  Layer.provide(WorkerServerLive),
  Layer.launch,
  Effect.ensuring(Console.log('Worker exited')),
);
