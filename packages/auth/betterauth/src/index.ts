import { connectNodeAdapter } from '@connectrpc/connect-node';
import * as NodeRuntime from '@effect/platform-node/NodeRuntime';
import { createClient } from '@libsql/client';
import { Config, Effect, Option, pipe } from 'effect';
import { createServer } from 'node:http';

import { AuthInternalService } from '@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb';

import { createAuth } from './auth.js';
import { initDatabase } from './db.js';
import { createInternalAuthService } from './service.js';

const oauthProvider = (idKey: string, secretKey: string) =>
  Effect.gen(function* () {
    const id = yield* Config.option(Config.string(idKey));
    const secret = yield* Config.option(Config.string(secretKey));
    if (Option.isSome(id) && Option.isSome(secret)) {
      return Option.some({ clientId: id.value, clientSecret: secret.value });
    }
    return Option.none();
  });

const oauthConfig = Effect.gen(function* () {
  const google = yield* oauthProvider('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_SECRET');
  const github = yield* oauthProvider('GITHUB_CLIENT_ID', 'GITHUB_CLIENT_SECRET');
  const microsoft = yield* oauthProvider('MICROSOFT_CLIENT_ID', 'MICROSOFT_CLIENT_SECRET');

  return {
    ...(Option.isSome(google) && { google: google.value }),
    ...(Option.isSome(github) && { github: github.value }),
    ...(Option.isSome(microsoft) && { microsoft: microsoft.value }),
  };
});

const program = Effect.gen(function* () {
  const port = yield* pipe(Config.integer('BETTERAUTH_PORT'), Config.withDefault(50051));
  const dbUrl = yield* pipe(Config.string('DATABASE_URL'), Config.withDefault('file:auth.db'));
  const dbAuthToken = yield* Config.option(Config.string('DATABASE_AUTH_TOKEN'));
  const authSecret = yield* pipe(
    Config.string('AUTH_SECRET'),
    Config.withDefault('development-auth-secret-change-in-production'),
  );
  const betterAuthUrl = yield* pipe(Config.string('BETTERAUTH_URL'), Config.withDefault(`http://localhost:${port}`));
  const oauth = yield* oauthConfig;

  const rawDb = createClient({
    authToken: Option.getOrUndefined(dbAuthToken),
    url: dbUrl,
  });

  const auth = createAuth(rawDb, {
    baseURL: betterAuthUrl,
    oauth,
    secret: authSecret,
  });

  const service = createInternalAuthService({ auth, rawDb });

  yield* Effect.tryPromise(() => initDatabase(rawDb));

  const handler = connectNodeAdapter({
    routes: (router) => {
      router.service(AuthInternalService, service);
    },
  });

  yield* Effect.acquireRelease(
    Effect.async<ReturnType<typeof createServer>, Error>((resume) => {
      const server = createServer(handler);
      server.on('error', (err) => {
        resume(Effect.fail(err));
      });
      server.listen(port, '0.0.0.0', () => {
        console.log(`[Auth] Service listening on :${port}`);
        resume(Effect.succeed(server));
      });
    }),
    (server) =>
      Effect.promise(
        () =>
          new Promise<void>((resolve) => {
            server.close(() => void resolve());
          }),
      ),
  );

  yield* Effect.never;
});

pipe(program, Effect.scoped, NodeRuntime.runMain);
