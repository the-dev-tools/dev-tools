import { createRouterTransport, Interceptor, Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Context, Effect, Layer, pipe, Runtime } from 'effect';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { CollectionService } from '@the-dev-tools/protobuf/collection/v1/collection_connect';
import { FlowService } from '@the-dev-tools/protobuf/flow/v1/flow_connect';

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

export const ApiTransportMock = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createRouterTransport(
      ({ service }) => {
        service(AuthService, {
          dID: (_) => ({ refreshToken: _.didToken }),
          accessToken: (_) => ({ accessToken: _.refreshToken }),
        });
        service(CollectionService, {
          createCollection: (_) => ({ id: _.name, name: _.name }),
          listCollections: () => ({ ids: [] }),
        });
        service(FlowService, {
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
