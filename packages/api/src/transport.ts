import {
  Code,
  ConnectError,
  createRouterTransport,
  StreamRequest,
  StreamResponse,
  Transport,
  UnaryRequest,
  UnaryResponse,
} from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Context, DateTime, Effect, flow, Layer, pipe, Runtime } from 'effect';
import * as Jose from 'jose';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { CollectionService } from '@the-dev-tools/protobuf/collection/v1/collection_connect';
import { FlowService } from '@the-dev-tools/protobuf/flow/v1/flow_connect';

import { AccessTokenPayload, RefreshTokenPayload } from './jwt';

export class ApiTransport extends Context.Tag('ApiTransport')<ApiTransport, Transport>() {}

export const ApiTransportDev = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      baseUrl: 'https://devtools-backend.fly.dev',
      useHttpGet: true,
      interceptors: [yield* effectInterceptor(authorizationInterceptor)],
    });
  }),
);

let mockEmailCount = 0;
const mockTokens = Effect.gen(function* () {
  const accessToken = yield* pipe(
    AccessTokenPayload.make({
      token_type: 'access_token',
      exp: pipe(yield* DateTime.now, DateTime.add({ hours: 2 }), DateTime.toDate),
      email: (++mockEmailCount).toString() + '@mock.com',
    }),
    Schema.encode(AccessTokenPayload),
    Effect.map((_) => new Jose.UnsecuredJWT(_).encode()),
  );

  const refreshToken = yield* pipe(
    RefreshTokenPayload.make({
      token_type: 'refresh_token',
      exp: pipe(yield* DateTime.now, DateTime.add({ days: 2 }), DateTime.toDate),
    }),
    Schema.encode(RefreshTokenPayload),
    Effect.map((_) => new Jose.UnsecuredJWT(_).encode()),
  );

  return { accessToken, refreshToken };
});

export const ApiTransportMock = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<KeyValueStore>();
    return createRouterTransport(
      (router) => {
        router.service(AuthService, {
          dID: () => Runtime.runPromise(runtime)(mockTokens),
          refreshToken: () => Runtime.runPromise(runtime)(mockTokens),
        });
        router.service(CollectionService, {
          listCollections: () => ({ metaCollections: [{ id: 'test', name: 'test' }] }),
          getCollection: (_) => ({
            id: _.id,
            name: _.id,
            item: [
              {
                itemData: {
                  case: 'itemApiCall',
                  value: {
                    id: _.id + '_api_call',
                    name: 'API Call #' + _.id,
                  },
                },
              },
            ],
          }),
          runApiCall: () => ({ status: 200 }),
        });
        router.service(FlowService, {
          create: (_) => ({ id: _.name, name: _.name }),
          save: () => ({}),
          load: (_) => ({ id: _.name, name: _.name, nodes: {}, vars: {} }),
          delete: () => ({}),
          addPostmanCollection: () => ({}),
        });
      },
      {
        transport: {
          interceptors: [
            yield* effectInterceptor(
              flow(
                // Interceptor flow order is reversed
                mockInterceptor,
                authorizationInterceptor,
              ),
            ),
          ],
        },
      },
    );
  }),
);

type Request = UnaryRequest | StreamRequest;
type Response = UnaryResponse | StreamResponse;
type AnyFn = (req: Request) => Promise<Response>;
type AnyFnEffect<E, R> = (req: Request) => Effect.Effect<Response, E, R>;

const finalizeEffectInterceptor = (next: AnyFn) => (request: Request) =>
  pipe(
    Effect.tryPromise({
      try: (_) => next({ ...request, signal: AbortSignal.any([_, request.signal]) }),
      catch: (_) => ConnectError.from(_),
    }),
    Effect.catchIf(
      (_) => _.code === Code.Canceled,
      () => Effect.interrupt,
    ),
  );

const effectInterceptor = <E, R>(
  interceptor: (next: ReturnType<typeof finalizeEffectInterceptor>) => AnyFnEffect<E, R>,
) =>
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<R>();
    return (next: AnyFn) => (request: Request) =>
      pipe(next, finalizeEffectInterceptor, interceptor, (_) => _(request), Runtime.runPromise(runtime));
  });

const authorizationInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    Effect.gen(function* () {
      if (request.service.typeName === AuthService.typeName) return yield* next(request);

      const store = yield* KeyValueStore;
      yield* pipe(
        store.forSchema(Schema.String).get('AccessToken'),
        Effect.flatten,
        Effect.tap((_) => void request.header.set('Authorization', `Bearer ${_}`)),
      );

      return yield* next(request);
    });

const mockInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    Effect.gen(function* () {
      const response = yield* next(request);
      yield* Effect.logDebug(`Mocking ${request.url}`, { request, response });
      yield* Effect.sleep('500 millis');
      return response;
    });
