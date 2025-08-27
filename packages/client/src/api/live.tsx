import { createConnectTransport } from '@connectrpc/connect-web';
import { Effect, Layer } from 'effect';
import { registry } from './registry';
import { ApiTransport, effectInterceptor, errorInterceptor } from './transport';

export const ApiLive = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      baseUrl: 'http://localhost:8080',
      // Interceptor flow order is reversed
      interceptors: [yield* effectInterceptor(errorInterceptor)],
      jsonOptions: { registry },
      useHttpGet: true,
    });
  }),
);
