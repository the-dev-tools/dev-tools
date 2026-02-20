import { jwtClient } from 'better-auth/client/plugins';
import { createAuthClient } from 'better-auth/react';
import { Config, Effect, pipe } from 'effect';
import { defaultUrl } from './config';

export const authClient = Effect.gen(function* () {
  const url = yield* pipe(Config.url('PUBLIC_AUTH_URL'), Config.withDefault(defaultUrl));

  return createAuthClient({
    baseURL: url.href,
    plugins: [jwtClient()],
  });
});
