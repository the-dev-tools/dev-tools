import {
  Code,
  ConnectError,
  StreamRequest,
  StreamResponse,
  Transport,
  UnaryRequest,
  UnaryResponse,
} from '@connectrpc/connect';
import { Context, Effect, Option, pipe, Runtime } from 'effect';

export class ApiTransport extends Context.Tag('ApiTransport')<ApiTransport, Transport>() {}

export class ApiErrorHandler extends Context.Tag('ApiErrorHandler')<ApiErrorHandler, (error: ConnectError) => void>() {}

export type Request = StreamRequest | UnaryRequest;
type Response = StreamResponse | UnaryResponse;
type AnyFn = (req: Request) => Promise<Response>;
export type AnyFnEffect<E, R> = (req: Request) => Effect.Effect<Response, E, R>;

export const finalizeEffectInterceptor = (next: AnyFn) => (request: Request) =>
  pipe(
    Effect.tryPromise({
      catch: (_) => ConnectError.from(_),
      try: (_) => next({ ...request, signal: AbortSignal.any([_, request.signal]) }),
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

export const errorInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    pipe(
      next(request),
      Effect.tapError(
        Effect.fn(function* (error) {
          const handler = yield* Effect.serviceOption(ApiErrorHandler);
          if (error instanceof ConnectError) Option.map(handler, (_) => void _(error));
        }),
      ),
    );
