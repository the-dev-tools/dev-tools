import { createRouterTransport, Interceptor, Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Context, DateTime, Effect, Layer, pipe, Runtime } from 'effect';
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
      interceptors: [yield* authorizationInterceptor],
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
          createCollection: (_) => ({ id: _.name, name: _.name }),
          listCollections: () => ({ simpleCollections: [{ id: 'test', name: 'test' }] }),
        });
        router.service(FlowService, {
          create: (_) => ({ id: _.name, name: _.name }),
          save: () => ({}),
          load: (_) => ({ id: _.name, name: _.name, nodes: {}, vars: {} }),
          delete: () => ({}),
          addPostmanCollection: () => ({}),
        });
      },
      { transport: { interceptors: [yield* authorizationInterceptor, yield* mockInterceptor] } },
    );
  }),
);

const authorizationInterceptor = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<KeyValueStore>();

  const interceptor: Interceptor = (next) => async (request) =>
    Effect.gen(function* () {
      if (request.service.typeName === AuthService.typeName) return next(request);

      const store = yield* KeyValueStore;
      yield* pipe(
        store.forSchema(Schema.String).get('AccessToken'),
        Effect.flatten,
        Effect.tap((_) => void request.header.set('Authorization', `Bearer ${_}`)),
      );

      return next(request);
    }).pipe(Runtime.runPromise(runtime));

  return interceptor;
});

const mockInterceptor = Effect.gen(function* () {
  const runtime = yield* Effect.runtime();

  const interceptor: Interceptor = (next) => async (request) =>
    Effect.gen(function* () {
      yield* Effect.logDebug('Sending message', request);
      yield* Effect.sleep('500 millis');
      return next(request);
    }).pipe(Runtime.runPromise(runtime));

  return interceptor;
});
