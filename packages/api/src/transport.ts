import { createRouterTransport, Interceptor, Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { Context, Effect, Layer } from 'effect';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { CollectionService } from '@the-dev-tools/protobuf/collection/v1/collection_connect';
import { FlowService } from '@the-dev-tools/protobuf/flow/v1/flow_connect';

export class ApiTransport extends Context.Tag('ApiTransport')<ApiTransport, Transport>() {}

export const ApiTransportDev = Layer.succeed(
  ApiTransport,
  ApiTransport.of(
    createConnectTransport({
      baseUrl: 'https://devtools-backend.fly.dev',
      useHttpGet: true,
    }),
  ),
);

const mockInterceptor: Interceptor = (next) => async (req) =>
  Effect.gen(function* () {
    yield* Effect.logDebug(`Sending message to ${req.url}`);
    yield* Effect.sleep('1 seconds');
    return yield* Effect.tryPromise(() => next(req));
  }).pipe(Effect.runPromise);

export const ApiTransportMock = Layer.succeed(
  ApiTransport,
  ApiTransport.of(
    createRouterTransport(
      ({ service }) => {
        service(AuthService, {
          dID: (_) => ({ token: _.didToken }),
        });
        service(CollectionService, {
          create: (_) => ({ id: _.name, name: _.name }),
          save: () => ({}),
          load: (_) => ({ id: _.id, name: _.id, nodes: [] }),
          delete: () => ({}),
          list: () => ({ ids: [] }),
          importPostman: (_) => ({ id: _.name }),
          move: () => ({}),
        });
        service(FlowService, {
          create: (_) => ({ id: _.name, name: _.name }),
          save: () => ({}),
          load: (_) => ({ id: _.name, name: _.name, nodes: {}, vars: {} }),
          delete: () => ({}),
          addPostmanCollection: () => ({}),
        });
      },
      { transport: { interceptors: [mockInterceptor] } },
    ),
  ),
);
