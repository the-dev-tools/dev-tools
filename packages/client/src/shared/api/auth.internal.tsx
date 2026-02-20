import { Effect } from 'effect';
import { authClient } from '@the-dev-tools/auth';
import { InterceptorNext, InterceptorRequest } from './connect-rpc';

export class AuthToken extends Effect.Service<AuthToken>()('AuthToken', {
  accessors: true,
  effect: Effect.gen(function* () {
    return {
      token: yield* Effect.gen(function* () {
        const auth = yield* authClient;
        const token = yield* Effect.promise(() => auth.token());
        return token.data?.token;
      }).pipe(Effect.cachedWithTTL('1 seconds')),
    };
  }),
}) {}

export const authInterceptor = Effect.fn(function* (next: InterceptorNext, request: InterceptorRequest) {
  const token = yield* AuthToken.token;
  if (token) request.header.set('Authorization', `Bearer ${token}`);
  return yield* Effect.tryPromise(() => next(request));
});
