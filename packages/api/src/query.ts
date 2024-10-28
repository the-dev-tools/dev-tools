import { create, DescMessage, isMessage, Message, MessageShape } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  useMutation as useConnectMutation,
  UseMutationOptions,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { Array, Function, HashMap, Option, pipe, Struct, Tuple } from 'effect';

import { EndpointListItem, EndpointListItemSchema } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import { ExampleListItem, ExampleListItemSchema } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import { FolderListItem, FolderListItemSchema } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import { CollectionItemSchema, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';

import {
  MutationSpec,
  SpecCompareItemFn,
  SpecCreateItemFn,
  SpecFnArgs,
  SpecOnSuccessFn,
  SpecQueryInputFn,
} from './query.internal';

const queryGetSetup = (args: SpecFnArgs) => {
  const {
    input,
    params: { query, queryInputFn },
    spec: { key },
    transport,
  } = args;

  if (query === undefined) return undefined;
  if (key === undefined) return undefined;

  const keyInput = pipe(
    HashMap.get(queryInputFnMap, queryInputFn),
    Option.map(Function.apply(args)),
    Option.getOrElse(() => Struct.pick(input, key)),
  );

  const queryKey = createConnectQueryKey({
    schema: query,
    input: keyInput,
    cardinality: 'finite',
    transport,
  });

  return { query, queryKey };
};

const queryGetAddCache: SpecOnSuccessFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { query, queryKey } = setup;

  queryClient.setQueryData(queryKey, create(query.output, { ...input, ...output }));
};

const queryGetUpdateCache: SpecOnSuccessFn = (args) => {
  const { input, output, queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { query, queryKey } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | Message) =>
    create(query.output, { ...old, ...input, ...output }),
  );
};

const queryGetDeleteCache: SpecOnSuccessFn = (args) => {
  const { queryClient } = args;

  const setup = queryGetSetup(args);
  if (setup === undefined) return;

  const { queryKey } = setup;

  queryClient.removeQueries({ queryKey, exact: true });
};

const queryListSetup = (args: SpecFnArgs) => {
  const {
    input,
    output,
    params: { query, queryInputFn, compareItemFn, createItemFn },
    spec: { parentKeys, key },
    transport,
  } = args;

  if (query === undefined) return;
  if (key === undefined) return;

  const itemField = query.output.field['items'];
  if (itemField?.fieldKind !== 'list' || itemField.message === undefined) return;

  const keyInput = pipe(
    HashMap.get(queryInputFnMap, queryInputFn),
    Option.map(Function.apply(args)),
    Option.getOrElse(() => Struct.pick(input, ...(parentKeys ?? []))),
  );

  const queryKey = createConnectQueryKey({
    schema: query,
    input: keyInput,
    cardinality: 'finite',
    transport,
  });

  const compareItem = pipe(
    HashMap.get(compareItemFnMap, compareItemFn),
    Option.map(Function.apply(args)),
    Option.getOrElse<ReturnType<SpecCompareItemFn>>(() => (old) => old[key] === input[key]),
  );

  const createItem = pipe(
    HashMap.get(createItemFnMap, createItemFn),
    Option.map(Function.apply(args)),
    Option.getOrElse<ReturnType<SpecCreateItemFn>>(
      () => (old) => create(itemField.message, { ...old, ...input, ...output }),
    ),
  );

  return {
    compareItem,
    createItem,
    query,
    queryKey,
  };
};

const queryListAddItemCache: SpecOnSuccessFn = (args) => {
  const { queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { createItem, query, queryKey } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: unknown[] })) =>
    create(query.output, { items: [...(old?.items ?? []), createItem()] }),
  );
};

const queryListUpdateItemCache: SpecOnSuccessFn = (args) => {
  const { queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { compareItem, createItem, query, queryKey } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: MessageShape<DescMessage>[] })) => {
    const oldItemIndex = old?.items.findIndex(compareItem);
    if (oldItemIndex === undefined) return old;
    return create(query.output, {
      items: Array.modify(old?.items ?? [], oldItemIndex, createItem),
    });
  });
};

const queryListDeleteItemCache: SpecOnSuccessFn = (args) => {
  const { queryClient } = args;

  const setup = queryListSetup(args);
  if (setup === undefined) return;

  const { compareItem, query, queryKey } = setup;

  queryClient.setQueryData(queryKey, (old: undefined | (Message & { items: MessageShape<DescMessage>[] })) => {
    const oldItemIndex = old?.items.findIndex(compareItem);
    if (oldItemIndex === undefined) return old;
    return create(query.output, {
      items: Array.remove(old?.items ?? [], oldItemIndex),
    });
  });
};

const queryInputCollectionItemList: SpecQueryInputFn = ({ input, spec: { parentKeys } }) => ({
  ...Struct.pick(input, ...(parentKeys ?? [])),
  folderId: input['parentFolderId'],
});

const compareItemCollectionItemEndpoint: SpecCompareItemFn =
  ({ input }) =>
  (old) => {
    if (!isMessage(old, CollectionItemSchema)) return false;
    return input['endpointId'] === old.endpoint?.endpointId;
  };

const createItemCollectionItemEndpoint: SpecCreateItemFn =
  ({ input, output }) =>
  (old) => {
    if (old !== undefined && !isMessage(old, CollectionItemSchema)) return old;
    const data = { ...input, ...output };
    return create(CollectionItemSchema, {
      ...old!,
      kind: ItemKind.ENDPOINT,
      endpoint: create(EndpointListItemSchema, data as EndpointListItem),
      example: create(ExampleListItemSchema, data as ExampleListItem),
    });
  };

const compareItemCollectionItemFolder: SpecCompareItemFn =
  ({ input }) =>
  (old) => {
    if (!isMessage(old, CollectionItemSchema)) return false;
    return input['folderId'] === old.folder?.folderId;
  };

const createItemCollectionItemFolder: SpecCreateItemFn =
  ({ input, output }) =>
  (old) => {
    if (old !== undefined && !isMessage(old, CollectionItemSchema)) return old;
    return create(CollectionItemSchema, {
      ...old!,
      kind: ItemKind.FOLDER,
      folder: create(FolderListItemSchema, { ...input, ...output } as FolderListItem),
    });
  };

export const queryInputFnMap = HashMap.make(['collection item - list', queryInputCollectionItemList]);

export const compareItemFnMap = HashMap.make(
  ['collection item - endpoint', compareItemCollectionItemEndpoint],
  ['collection item - folder', compareItemCollectionItemFolder],
);

export const createItemFnMap = HashMap.make(
  ['collection item - endpoint', createItemCollectionItemEndpoint],
  ['collection item - folder', createItemCollectionItemFolder],
);

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
