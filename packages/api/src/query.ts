import { create, DescMessage, Message } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  useMutation as useConnectMutation,
  UseMutationOptions,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { Array, HashMap, Option, pipe, Struct, Tuple } from 'effect';

import { MutationSpec, SpecFn, SpecFnArgs } from './query.internal';

const queryGetSetup = ({ input, params: { query }, spec: { key }, transport }: SpecFnArgs) => {
  if (query === undefined) return undefined;
  if (key === undefined) return undefined;

  const keyInput = Struct.pick(input, key);

  const queryKey = createConnectQueryKey({
    schema: query,
    input: keyInput,
    cardinality: 'finite',
    transport,
  });

  return { query, queryKey };
};

const queryGetAddCache: SpecFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { query, queryKey } = setup;

  queryClient.setQueryData(queryKey, create(query.output, { ...input, ...output }));
};

const queryGetUpdateCache: SpecFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { query, queryKey } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | Message) =>
    create(query.output, { ...old, ...input, ...output }),
  );
};

const queryGetDeleteCache: SpecFn = (args) => {
  const { queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { queryKey } = setup;

  queryClient.removeQueries({ queryKey, exact: true });
};

const queryListSetup = ({ input, params: { query }, spec: { parentKeys, key }, transport }: SpecFnArgs) => {
  if (query === undefined) return;
  if (key === undefined) return;

  const itemField = query.output.field['items'];
  if (itemField?.fieldKind !== 'list' || itemField.message === undefined) return;

  const keyInput = Struct.pick(input, ...(parentKeys ?? []));

  const queryKey = createConnectQueryKey({
    schema: query,
    input: keyInput,
    cardinality: 'finite',
    transport,
  });

  return { query, queryKey, key, itemSchema: itemField.message };
};

const queryListAddItemCache: SpecFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { query, queryKey, itemSchema } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: unknown[] })) => {
    const item = create(itemSchema, { ...input, ...output });
    return create(query.output, { items: [...(old?.items ?? []), item] });
  });
};

const queryListUpdateItemCache: SpecFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { query, queryKey, key, itemSchema } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: { [Key in typeof key]: unknown }[] })) => {
    const oldItemIndex = old?.items.findIndex((old) => old[key] === input[key]);
    if (oldItemIndex === undefined) return old;
    return create(query.output, {
      items: Array.modify(old?.items ?? [], oldItemIndex, (old) => create(itemSchema, { ...old, ...input, ...output })),
    });
  });
};

const queryListDeleteItemCache: SpecFn = (args) => {
  const { input, queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { query, queryKey, key } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: { [Key in typeof key]: unknown }[] })) =>
    create(query.output, {
      items: Array.filter(old?.items ?? [], (old) => old[key] !== input[key]),
    }),
  );
};

export const onSuccessMap = HashMap.make(
  ['query - get - add cache', queryGetAddCache],
  ['query - get - update cache', queryGetUpdateCache],
  ['query - get - delete cache', queryGetDeleteCache],

  ['query - list - add item cache', queryListAddItemCache],
  ['query - list - update item cache', queryListUpdateItemCache],
  ['query - list - delete item cache', queryListDeleteItemCache],
);

export const useSpecMutation = <Input extends DescMessage, Output extends DescMessage, Context = unknown>(
  spec: MutationSpec<Input, Output>,
  queryOptions?: UseMutationOptions<Input, Output, Context>,
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  return useConnectMutation(spec.mutation, {
    ...queryOptions,
    onSuccess: (output, input, context) => {
      queryOptions?.onSuccess?.(output, input, context);

      const args = {
        input,
        output,
        queryClient,
        spec,
        transport,
      };

      pipe(
        spec.onSuccess ?? [],
        Array.map(([key, params]) =>
          pipe(
            HashMap.get(onSuccessMap, key),
            Option.map((fn) => Tuple.make(fn, params)),
          ),
        ),
        Array.getSomes,
        Array.forEach(([fn, params]) => void fn({ ...args, params })),
      );
    },
  });
};
