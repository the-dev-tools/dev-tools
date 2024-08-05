import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Config, Context, Effect, Layer, pipe } from 'effect';
import { decodeJwt } from 'jose';
import { LoginWithMagicLinkConfiguration, Magic, PromiEvent } from 'magic-sdk';

import { ApiClient } from './client';
import { accessTokenKey, AccessTokenPayload, refreshTokenKey, RefreshTokenPayload } from './jwt';

class MagicClient extends Context.Tag('MagicClient')<MagicClient, Magic>() {}

export const MagicClientLive = Layer.effect(
  MagicClient,
  Effect.gen(function* () {
    const apiKey = yield* Config.string('PUBLIC_MAGIC_KEY');
    return new Magic(apiKey, {
      useStorageCache: true,
      deferPreload: true,
    });
  }),
);

export const MagicClientMock = Layer.succeed(MagicClient, {
  auth: {
    loginWithMagicLink: () => Promise.resolve('mock-did-token') as PromiEvent<string>,
  } as Partial<Magic['auth']>,
  user: {
    logout: () => Promise.resolve(true),
  },
} as Magic);

export const login = (configuration: LoginWithMagicLinkConfiguration) =>
  Effect.gen(function* () {
    // Authenticate using Magic SDK
    const magicClient = yield* MagicClient;
    const didToken = yield* pipe(
      Effect.tryPromise(() => magicClient.auth.loginWithMagicLink(configuration)),
      Effect.flatMap(Effect.fromNullable),
    );

    // Authorize
    const apiClient = yield* ApiClient;
    const { accessToken, refreshToken } = (yield* apiClient.auth.dID({ didToken })).message;

    // Validate tokens
    yield* pipe(
      Effect.try(() => decodeJwt<typeof AccessTokenPayload.Encoded>(accessToken)),
      Effect.flatMap(Schema.decode(AccessTokenPayload)),
    );
    yield* pipe(
      Effect.try(() => decodeJwt<typeof RefreshTokenPayload.Encoded>(refreshToken)),
      Effect.flatMap(Schema.decode(RefreshTokenPayload)),
    );

    // Store tokens
    const store = yield* KeyValueStore;
    yield* store.forSchema(Schema.String).set(accessTokenKey, accessToken);
    yield* store.forSchema(Schema.String).set(refreshTokenKey, refreshToken);
  });

export const logout = Effect.gen(function* () {
  const magicClient = yield* MagicClient;
  yield* Effect.tryPromise(() => magicClient.user.logout());
  const store = yield* KeyValueStore;
  yield* store.remove(accessTokenKey);
  yield* store.remove(refreshTokenKey);
});

export const getUser = pipe(
  KeyValueStore,
  Effect.flatMap((_) => _.forSchema(Schema.String).get(accessTokenKey)),
  Effect.flatten,
  Effect.flatMap((_) => Effect.try(() => decodeJwt<typeof AccessTokenPayload.Encoded>(_))),
  Effect.flatMap(Schema.decode(AccessTokenPayload)),
);
