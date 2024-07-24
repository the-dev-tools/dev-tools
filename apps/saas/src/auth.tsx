import { Effect } from 'effect';
import { LoginWithMagicLinkConfiguration, Magic } from 'magic-sdk';

export const magicLink = new Magic('pk_live_75E3754872D9F513', {
  useStorageCache: true,
});

export const login = (configuration: LoginWithMagicLinkConfiguration) =>
  Effect.tryPromise(() => magicLink.auth.loginWithMagicLink(configuration));

export const logout = Effect.tryPromise(() => magicLink.user.logout());

export const isLoggedIn = Effect.tryPromise(() => magicLink.user.isLoggedIn());

export const getInfo = Effect.tryPromise(() => magicLink.user.getInfo());
