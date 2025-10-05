import { DescMessage, DescMethodStreaming, DescMethodUnary, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { ConnectError, createContextValues, Transport } from '@connectrpc/connect';
import { ConnectQueryKey, createConnectQueryKey, UseMutationOptions, useTransport } from '@connectrpc/connect-query';
import { AnyDataTag, DataTag, SkipToken, useMutation, UseMutationResult } from '@tanstack/react-query';
import { useCallback } from 'react';

import { enableErrorInterceptorKey } from './transport';

export {
  useInfiniteQuery as useConnectInfiniteQuery,
  useQuery as useConnectQuery,
  useSuspenseInfiniteQuery as useConnectSuspenseInfiniteQuery,
  useSuspenseQuery as useConnectSuspenseQuery,
} from '@connectrpc/connect-query';

// Customized Connect TanStack Query wrapper to enable error interceptor and
// add schema to meta
// https://github.com/connectrpc/connect-query-es/blob/main/packages/connect-query/src/use-mutation.ts
export function useConnectMutation<I extends DescMessage, O extends DescMessage, Ctx = unknown>(
  schema: DescMethodUnary<I, O>,
  { transport, ...queryOptions }: UseMutationOptions<I, O, Ctx> = {},
): UseMutationResult<MessageShape<O>, ConnectError, MessageInitShape<I>, Ctx> {
  const transportFromCtx = useTransport();
  const transportToUse = transport ?? transportFromCtx;

  const mutationFn = useCallback(
    async (input: MessageInitShape<I>) => {
      const response = await transportToUse.unary(
        schema,
        undefined,
        undefined,
        undefined,
        input,
        createContextValues().set(enableErrorInterceptorKey, true),
      );
      return response.message;
    },
    [transportToUse, schema],
  );

  return useMutation({
    ...queryOptions,
    meta: {
      schema,
      ...queryOptions.meta,
    },
    mutationFn,
  });
}

export type ConnectStreamingQueryKey<O extends DescMessage> = DataTag<
  Omit<ConnectQueryKey, keyof AnyDataTag>,
  O[],
  ConnectError
>;

// TODO: replace with an official solution once implemented upstream
// https://github.com/connectrpc/connect-query-es/issues/524
export const createConnectStreamingQueryKey = <I extends DescMessage, O extends DescMessage>(params: {
  input?: MessageInitShape<I> | SkipToken | undefined;
  schema: DescMethodStreaming<I, O>;
  transport?: Transport;
}) => createConnectQueryKey({ ...params, cardinality: 'finite' } as never) as unknown as ConnectStreamingQueryKey<O>;
