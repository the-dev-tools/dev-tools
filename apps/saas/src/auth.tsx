import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Effect, pipe } from 'effect';
import { decodeJwt } from 'jose';
import { LoginWithMagicLinkConfiguration, Magic } from 'magic-sdk';

import { ApiClient } from '@the-dev-tools/api/client';

export const magicLink = new Magic('pk_live_75E3754872D9F513', {
  useStorageCache: true,
  deferPreload: true,
});

class JWTPayload extends Schema.Class<JWTPayload>('JWTPayload')({
  email: Schema.String,
  exp: Schema.transform(Schema.Number, Schema.DateFromSelf, {
    strict: true,
    decode: (_) => new Date(_ * 1000),
    encode: (_) => Math.floor(_.getTime() / 1000),
  }),
}) {}

const accessTokenKey = 'AccessToken';
class AccessTokenPayload extends JWTPayload.extend<AccessTokenPayload>('AccessTokenPayload')({
  token_type: Schema.Literal('access_token'),
}) {}

const refreshTokenKey = 'RefreshToken';
class RefreshTokenPayload extends JWTPayload.extend<RefreshTokenPayload>('RefreshTokenPayload')({
  token_type: Schema.Literal('refresh_token'),
}) {}

export const login = (configuration: LoginWithMagicLinkConfiguration) =>
  Effect.gen(function* () {
    // Authenticate using Magic SDK
    const didToken = yield* pipe(
      Effect.tryPromise(() => magicLink.auth.loginWithMagicLink(configuration)),
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
  yield* Effect.tryPromise(() => magicLink.user.logout());
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
