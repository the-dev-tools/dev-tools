import { CallOptions, Interceptor, Transport } from '@connectrpc/connect';
import { Effect, identity, pipe, Runtime } from 'effect';
import * as Protobuf from './protobuf';

export * from '@connectrpc/connect';
export * from '@connectrpc/connect-web';

interface SimpleCallOptions<I extends Protobuf.DescMessage> extends Omit<CallOptions, 'onHeader' | 'onTrailer'> {
  input?: Protobuf.MessageInitShape<I>;
  transport: Transport;
}

export interface RequestOptions<
  I extends Protobuf.DescMessage,
  O extends Protobuf.DescMessage,
> extends SimpleCallOptions<I> {
  method: Protobuf.DescMethodUnary<I, O>;
}

export const request = <I extends Protobuf.DescMessage, O extends Protobuf.DescMessage>(_: RequestOptions<I, O>) =>
  _.transport.unary(
    _.method,
    _.signal,
    _.timeoutMs,
    _.headers,
    _.input ?? ({} as Protobuf.MessageInitShape<I>),
    _.contextValues,
  );

export interface StreamOptions<
  I extends Protobuf.DescMessage,
  O extends Protobuf.DescMessage,
> extends SimpleCallOptions<I> {
  method: Protobuf.DescMethodServerStreaming<I, O>;
}

export async function* stream<I extends Protobuf.DescMessage, O extends Protobuf.DescMessage>(_: StreamOptions<I, O>) {
  const response = await _.transport.stream(
    _.method,
    _.signal,
    _.timeoutMs,
    _.headers,
    createAsyncIterable([_.input ?? ({} as Protobuf.MessageInitShape<I>)]),
    _.contextValues,
  );

  yield* response.message;
}

// eslint-disable-next-line @typescript-eslint/require-await
async function* createAsyncIterable<T>(items: T[]): AsyncIterable<T> {
  yield* items;
}

export type InterceptorNext = Parameters<Interceptor>[0];
export type InterceptorRequest = Parameters<Parameters<Interceptor>[0]>[0];
export type InterceptorResponse = Awaited<ReturnType<Parameters<Interceptor>[0]>>;

export const effectInterceptor = Effect.fn(function* <E, R>(
  interceptor: (next: InterceptorNext, request: InterceptorRequest) => Effect.Effect<InterceptorResponse, E, R>,
) {
  const runtime = yield* Effect.runtime<R>();
  return identity<Interceptor>((next) => (request) => pipe(interceptor(next, request), Runtime.runPromise(runtime)));
});
