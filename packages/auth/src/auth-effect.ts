import { Path } from '@effect/platform';
import { betterAuth, type BetterAuthPlugin } from 'better-auth';
import { jwt, organization } from 'better-auth/plugins';
import { Config, Effect, pipe, Redacted } from 'effect';
import os from 'node:os';
import { createAdapter } from './adapter.ts';
import { defaultUrl } from './config.ts';

export const plugins = [
  jwt({
    jwks: {
      keyPairConfig: { alg: 'RS256' },
    },
    jwt: {
      definePayload: ({ session, user }) => ({
        email: user.email,
        expiresAt: session.expiresAt,
        name: user.name,
        userId: user.id,
      }),
    },
  }),
] satisfies BetterAuthPlugin[];

export const authEffect = Effect.gen(function* () {
  const path = yield* Path.Path;

  const configNamespace = Config.nested('AUTH');

  const url = yield* pipe(Config.url('URL'), configNamespace, Config.withDefault(defaultUrl));

  const secret = yield* pipe(Config.redacted('SECRET'), configNamespace);

  const adapterSocketPath = yield* pipe(
    Config.string('AUTH_ADAPTER_SOCKET'),
    Config.withDefault(path.resolve(os.tmpdir(), 'the-dev-tools', 'server.socket')),
  );

  return betterAuth({
    baseURL: url.href,
    database: createAdapter({ socketPath: adapterSocketPath }),
    emailAndPassword: { enabled: true, requireEmailVerification: false },
    plugins: [
      ...plugins,
      organization({ creatorRole: 'owner' }),
    ],
    secret: Redacted.value(secret),
    session: {
      expiresIn: 60 * 60 * 24 * 7, // 7 days
      updateAge: 60 * 60 * 24, // update session every day
    },
    trustedOrigins: ['*'],
  });
});
