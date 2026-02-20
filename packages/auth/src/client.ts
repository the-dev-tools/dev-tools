import { jwtClient } from 'better-auth/client/plugins';
import { createAuthClient } from 'better-auth/react';
import { Config, Effect, pipe } from 'effect';
import { defaultUrl } from './config';

const accessTokenKey = 'ACCESS_TOKEN';

export const authClient = Effect.gen(function* () {
  const url = yield* pipe(Config.url('PUBLIC_AUTH_URL'), Config.withDefault(defaultUrl));

  return createAuthClient({
    baseURL: url.href,

    plugins: [jwtClient()],

    fetchOptions: {
      auth: {
        token: () => localStorage.getItem(accessTokenKey) ?? undefined,
        type: 'Bearer',
      },
      onSuccess: (ctx) => {
        const authToken = ctx.response.headers.get('set-auth-token');
        if (authToken) localStorage.setItem(accessTokenKey, authToken);
      },
    },
  });
});
