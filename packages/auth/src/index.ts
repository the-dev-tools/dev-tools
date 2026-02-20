import { HttpApp, HttpMiddleware, HttpRouter, HttpServer } from '@effect/platform';
import { NodeHttpServer, NodeRuntime } from '@effect/platform-node';
import { Effect, Layer, pipe } from 'effect';
import { createServer } from 'node:http';
import { authEffect } from './auth-effect.ts';

const app = Effect.gen(function* () {
  const auth = yield* authEffect;
  const authHttpApp = HttpApp.fromWebHandler(auth.handler);

  return pipe(
    HttpRouter.empty,
    HttpRouter.mountApp('/api/auth', authHttpApp, { includePrefix: true }),
    HttpMiddleware.logger,
    HttpMiddleware.cors({ allowedOrigins: () => true, credentials: true }),
    HttpServer.serve(),
    HttpServer.withLogAddress,
  );
});

const HttpServerLive = NodeHttpServer.layer(() => createServer(), { port: 5000 });

pipe(Layer.unwrapEffect(app), Layer.provide(HttpServerLive), Layer.launch, NodeRuntime.runMain);
