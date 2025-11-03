import { CallOptions, Transport } from '@connectrpc/connect';
import * as Protobuf from './protobuf';

export * from '@connectrpc/connect';

interface SimpleCallOptions<I extends Protobuf.DescMessage> extends Omit<CallOptions, 'onHeader' | 'onTrailer'> {
  input?: Protobuf.MessageInitShape<I>;
  transport: Transport;
}

export interface RequestOptions<I extends Protobuf.DescMessage, O extends Protobuf.DescMessage>
  extends SimpleCallOptions<I> {
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

export interface StreamOptions<I extends Protobuf.DescMessage, O extends Protobuf.DescMessage>
  extends SimpleCallOptions<I> {
  method: Protobuf.DescMethodServerStreaming<I, O>;
}

export const stream = <I extends Protobuf.DescMessage, O extends Protobuf.DescMessage>(_: StreamOptions<I, O>) =>
  _.transport.stream(
    _.method,
    _.signal,
    _.timeoutMs,
    _.headers,
    createAsyncIterable([_.input ?? ({} as Protobuf.MessageInitShape<I>)]),
    _.contextValues,
  );

// eslint-disable-next-line @typescript-eslint/require-await
async function* createAsyncIterable<T>(items: T[]): AsyncIterable<T> {
  yield* items;
}
