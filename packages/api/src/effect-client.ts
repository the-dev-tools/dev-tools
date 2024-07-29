import { Message, MethodInfo, MethodInfoUnary, MethodKind, PartialMessage, ServiceType } from '@bufbuild/protobuf';
import { ConnectError, makeAnyClient } from '@connectrpc/connect';
import { Effect, Match, pipe } from 'effect';

import { ApiTransport } from './transport';

interface CallOptions {
  timeoutMs?: number;
  headers?: HeadersInit;
}

const createUnaryFn =
  <I extends Message<I>, O extends Message<O>>(service: ServiceType, method: MethodInfo<I, O>) =>
  (request: PartialMessage<I>, options?: CallOptions) =>
    Effect.tryMapPromise(ApiTransport, {
      try: (transport, signal) =>
        transport.unary(service, method, signal, options?.timeoutMs, options?.headers, request),
      catch: (_) => ConnectError.from(_),
    });

export type EffectClient<T extends ServiceType> = {
  [P in keyof T['methods']]: T['methods'][P] extends MethodInfoUnary<infer I, infer O>
    ? ReturnType<typeof createUnaryFn<I, O>>
    : never;
};

export const createEffectClient = <T extends ServiceType>(service: T) =>
  makeAnyClient(service, (method) =>
    pipe(
      Match.value(method.kind),
      Match.when(MethodKind.Unary, () => createUnaryFn(service, method)),
      Match.orElseAbsurd,
    ),
  ) as EffectClient<T>;
