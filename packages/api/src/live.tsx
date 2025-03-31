import { ConnectTransportOptions, createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, flow, Layer, pipe } from 'effect';
import { Magic } from 'magic-sdk';

import { authorizationInterceptor, AuthTransport, MagicClient } from './auth';
import { LocalMode } from './local';
import { registry } from './meta';
import { ApiTransport, effectInterceptor, errorInterceptor } from './transport';

const baseTransportOptions = Effect.gen(function* () {
  return {
    baseUrl: (yield* LocalMode) ? 'http://localhost:8080' : yield* Config.string('PUBLIC_API_URL'),
    jsonOptions: { registry },
    useHttpGet: true,
  } satisfies ConnectTransportOptions;
});

const AuthTransportLive = Layer.effect(
  AuthTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      ...(yield* baseTransportOptions),
      interceptors: [yield* effectInterceptor(errorInterceptor)],
    });
  }),
);

const MagicClientLive = Layer.effect(
  MagicClient,
  Effect.gen(function* () {
    // TODO: improve auth layer composition for local mode
    if (yield* LocalMode) return {} as Magic;
    const apiKey = yield* Config.string('PUBLIC_MAGIC_KEY');
    return new Magic(apiKey, {
      deferPreload: true,
      useStorageCache: true,
    });
  }),
);

const ApiTransportLive = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      ...(yield* baseTransportOptions),
      // Interceptor flow order is reversed
      interceptors: [yield* effectInterceptor(flow(errorInterceptor, authorizationInterceptor))],
    });
  }),
);

export const ApiLive = pipe(
  ApiTransportLive,
  Layer.provideMerge(AuthTransportLive),
  Layer.provideMerge(MagicClientLive),
);
