import type { IncomingMessage, ServerResponse } from 'node:http';
import { connectNodeAdapter } from '@connectrpc/connect-node';
import * as NodeRuntime from '@effect/platform-node/NodeRuntime';
import { createClient } from '@libsql/client';
import { Config, Effect, Option, pipe } from 'effect';
import { createServer } from 'node:http';
import { AuthService } from '@the-dev-tools/spec/buf/api/internal/auth/v1/auth_pb';
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

  return {
    ...(Option.isSome(google) && { google: google.value }),
  };
});

const program = Effect.gen(function* () {
  const port = yield* pipe(Config.integer('BETTERAUTH_PORT'), Config.withDefault(50051));
  const host = yield* pipe(Config.string('HOST'), Config.withDefault('127.0.0.1'));
  const dbUrl = yield* pipe(Config.string('DATABASE_URL'), Config.withDefault('file:auth.db'));
  const dbAuthToken = yield* Config.option(Config.string('DATABASE_AUTH_TOKEN'));
  const authSecret = yield* Config.string('AUTH_SECRET');
  const betterAuthUrl = yield* pipe(Config.string('BETTERAUTH_URL'), Config.withDefault(`http://localhost:${port}`));
  const oauth = yield* oauthConfig;

  const rawDb = createClient({
    authToken: Option.getOrUndefined(dbAuthToken),
    url: dbUrl,
  });

  yield* Effect.tryPromise({
    catch: (e) => new Error(`Database initialization failed: ${String(e)}`),
    try: () => initDatabase(rawDb),
  });

  const auth = createAuth(rawDb, {
    baseURL: betterAuthUrl,
    oauth,
    secret: authSecret,
  });

  const service = createInternalAuthService({ auth, rawDb });

  const rpcHandler = connectNodeAdapter({
    routes: (router) => {
      router.service(AuthService, service);
    },
  });

  // Combined handler: BetterAuth HTTP routes (e.g. /api/auth/jwks) + Connect RPC
  const handler = (req: IncomingMessage, res: ServerResponse) => {
    if (req.url?.startsWith('/api/auth/')) {
      // Convert Node.js request to Web Request for BetterAuth
      const protocol = 'http';
      const url = `${protocol}://${req.headers.host ?? 'localhost'}${req.url}`;
      const headers = new Headers();
      for (const [key, value] of Object.entries(req.headers)) {
        if (value) headers.set(key, Array.isArray(value) ? value.join(', ') : value);
      }
      const webReq = new Request(url, { headers, method: req.method ?? 'GET' });
      auth
        .handler(webReq)
        .then(async (webRes: Response) => {
          res.writeHead(webRes.status, Object.fromEntries(webRes.headers.entries()));
          const body = await webRes.arrayBuffer();
          res.end(Buffer.from(body));
        })
        .catch(() => {
          res.writeHead(500);
          res.end();
        });
      return;
    }
    rpcHandler(req, res);
  };

  yield* Effect.acquireRelease(
    Effect.async<ReturnType<typeof createServer>, Error>((resume) => {
      const server = createServer(handler);
      server.on('error', (err) => {
        resume(Effect.fail(err));
      });
      server.listen(port, host, () => {
        console.log(`[Auth] Service listening on ${host}:${port}`);
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
