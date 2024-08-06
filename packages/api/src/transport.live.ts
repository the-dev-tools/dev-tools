import { createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, Layer } from 'effect';

import { ApiTransport, authorizationInterceptor, effectInterceptor } from './transport';

export const ApiTransportLive = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      baseUrl: yield* Config.string('PUBLIC_API_URL'),
      useHttpGet: true,
      interceptors: [yield* effectInterceptor(authorizationInterceptor)],
    });
  }),
);
