import { Schema } from '@effect/schema';
import { Effect } from 'effect';
import { Magic } from 'magic-sdk';

import * as Storage from '@/storage';

const magicLink = new Magic('pk_live_75E3754872D9F513', {
  deferPreload: true,
  useStorageCache: true,
});

const LoggedInTag = 'LoggedInTag';
const LoggedIn = Schema.Boolean;

const setLoggedIn = Storage.set(Storage.Local, LoggedInTag, LoggedIn);

export const useLoggedIn = () => Storage.useState(Storage.Local, LoggedInTag, LoggedIn);

const CallbackTab = 'auth-callback';

export const loginInit = (email: string) =>
  Effect.gen(function* () {
    const result = yield* Effect.promise(() =>
      magicLink.auth.loginWithMagicLink({
        email,
        redirectURI: `chrome-extension://${chrome.runtime.id}/tabs/${CallbackTab}.html`,
      }),
    );
    if (result === null) return false;
    yield* setLoggedIn(true);
    return true;
  });

export const loginConfirm = (token: string) =>
  Effect.gen(function* () {
    const result = yield* Effect.promise(() => magicLink.auth.loginWithCredential(token));
    if (result === null) return false;
    yield* setLoggedIn(true);
    return true;
  });

export const logout = Effect.gen(function* () {
  const result = yield* Effect.promise(() => magicLink.user.logout());
  if (!result) return false;
  yield* setLoggedIn(false);
  return true;
});
