import {
  Code,
  ConnectError,
  StreamRequest,
  StreamResponse,
  Transport,
  UnaryRequest,
  UnaryResponse,
} from '@connectrpc/connect';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Context, Effect, pipe, Runtime } from 'effect';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';

export class ApiTransport extends Context.Tag('ApiTransport')<ApiTransport, Transport>() {}

export type Request = UnaryRequest | StreamRequest;
type Response = UnaryResponse | StreamResponse;
type AnyFn = (req: Request) => Promise<Response>;
export type AnyFnEffect<E, R> = (req: Request) => Effect.Effect<Response, E, R>;

export const finalizeEffectInterceptor = (next: AnyFn) => (request: Request) =>
  pipe(
    Effect.tryPromise({
      try: (_) => next({ ...request, signal: AbortSignal.any([_, request.signal]) }),
      catch: (_) => ConnectError.from(_),
    }),
    Effect.catchIf(
      (_) => _.code === Code.Canceled,
      () => Effect.interrupt,
    ),
  );

export const effectInterceptor = <E, R>(
  interceptor: (next: ReturnType<typeof finalizeEffectInterceptor>) => AnyFnEffect<E, R>,
) =>
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<R>();
    return (next: AnyFn) => (request: Request) =>
      pipe(next, finalizeEffectInterceptor, interceptor, (_) => _(request), Runtime.runPromise(runtime));
  });

export const authorizationInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    Effect.gen(function* () {
      if (request.service.typeName === AuthService.typeName) return yield* next(request);

      const store = yield* KeyValueStore;
      yield* pipe(
        store.forSchema(Schema.String).get('AccessToken'),
        Effect.flatten,
        Effect.tap((_) => void request.header.set('Authorization', `Bearer ${_}`)),
      );

      return yield* next(request);
    });
