import { ConnectTransportOptions, createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, Layer, pipe } from 'effect';
import { Magic } from 'magic-sdk';

import { authorizationInterceptor, AuthTransport, MagicClient } from './auth';
import { registry } from './meta';
import { ApiTransport, effectInterceptor } from './transport';

const baseTransportOptions = Effect.gen(function* () {
  return {
    baseUrl: yield* Config.string('PUBLIC_API_URL'),
    useHttpGet: true,
    jsonOptions: { registry },
  } satisfies ConnectTransportOptions;
});

const AuthTransportLive = Layer.effect(
  AuthTransport,
  Effect.gen(function* () {
    return createConnectTransport(yield* baseTransportOptions);
  }),
);

const MagicClientLive = Layer.effect(
  MagicClient,
  Effect.gen(function* () {
    const apiKey = yield* Config.string('PUBLIC_MAGIC_KEY');
    return new Magic(apiKey, {
      useStorageCache: true,
      deferPreload: true,
    });
  }),
);

const ApiTransportLive = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    return createConnectTransport({
      ...(yield* baseTransportOptions),
      interceptors: [yield* effectInterceptor(authorizationInterceptor)],
    });
  }),
);

export const ApiLive = pipe(
  ApiTransportLive,
  Layer.provideMerge(AuthTransportLive),
  Layer.provideMerge(MagicClientLive),
);
