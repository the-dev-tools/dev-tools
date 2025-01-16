import { DescMessage, DescMethodUnary, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { ConnectError } from '@connectrpc/connect';
import { callUnaryMethod, UseMutationOptions, useTransport } from '@connectrpc/connect-query';
import { useMutation, UseMutationResult } from '@tanstack/react-query';
import { useCallback } from 'react';

export {
  useQuery as useConnectQuery,
  useInfiniteQuery as useConnectInfiniteQuery,
  useSuspenseInfiniteQuery as useConnectSuspenseInfiniteQuery,
  useSuspenseQuery as useConnectSuspenseQuery,
} from '@connectrpc/connect-query';

// Customized Connect TanStack Query wrapper to add schema to meta
// https://github.com/connectrpc/connect-query-es/blob/main/packages/connect-query/src/use-mutation.ts
export function useConnectMutation<I extends DescMessage, O extends DescMessage, Ctx = unknown>(
  schema: DescMethodUnary<I, O>,
  { transport, ...queryOptions }: UseMutationOptions<I, O, Ctx> = {},
): UseMutationResult<MessageShape<O>, ConnectError, MessageInitShape<I>, Ctx> {
  const transportFromCtx = useTransport();
  const transportToUse = transport ?? transportFromCtx;
  const mutationFn = useCallback(
    async (input: MessageInitShape<I>) => callUnaryMethod(transportToUse, schema, input),
    [transportToUse, schema],
  );
  return useMutation({
    ...queryOptions,
    mutationFn,
    meta: {
      schema,
      ...queryOptions.meta,
    },
  });
}
