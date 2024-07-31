import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Effect, pipe } from 'effect';
import { LoginWithMagicLinkConfiguration, Magic } from 'magic-sdk';

import { ApiClient } from '@the-dev-tools/api/client';

export const magicLink = new Magic('pk_live_75E3754872D9F513', {
  useStorageCache: true,
  deferPreload: true,
});

const accessTokenKey = 'AccessToken';

export const login = (configuration: LoginWithMagicLinkConfiguration) =>
  Effect.gen(function* () {
    const didToken = yield* pipe(
      Effect.tryPromise(() => magicLink.auth.loginWithMagicLink(configuration)),
      Effect.flatMap(Effect.fromNullable),
    );
    const apiClient = yield* ApiClient;
    const response = yield* apiClient.auth.dID({ didToken });
    const store = yield* KeyValueStore;
    yield* store.forSchema(Schema.String).set(accessTokenKey, response.message.token);
  });

export const logout = Effect.gen(function* () {
  yield* Effect.tryPromise(() => magicLink.user.logout());
  const store = yield* KeyValueStore;
  yield* store.remove(accessTokenKey);
});

export const getUser = Effect.gen(function* () {
  const store = yield* KeyValueStore;
  const jwt = yield* store.forSchema(Schema.String).get(accessTokenKey).pipe(Effect.flatten);
  // TODO: decode JWT payload
  return { jwt };
});
