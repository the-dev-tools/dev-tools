import { DescMessage, DescMethodUnary, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { ConnectError, createContextValues } from '@connectrpc/connect';
import { UseMutationOptions, useTransport } from '@connectrpc/connect-query';
import { useMutation, UseMutationResult } from '@tanstack/react-query';
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
