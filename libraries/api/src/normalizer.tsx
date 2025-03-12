import { create, DescMethodUnary, fromJson, isMessage, JsonObject, Message, toJson } from '@bufbuild/protobuf';
import { anyPack, AnySchema, anyUnpack } from '@bufbuild/protobuf/wkt';
import { ConnectQueryKey } from '@connectrpc/connect-query';
import { createNormalizer, Data } from '@normy/core';
import { Query, QueryClient, QueryKey, Updater } from '@tanstack/react-query';
import { Array, Boolean, flow, Option, pipe, Predicate, Record, Struct } from 'effect';
import { createContext, ReactNode, useContext, useEffect, useState } from 'react';

import {
  Change,
  ChangeKind,
  ChangeSchema,
  ListChange,
  ListChangeKind,
  ListChangeSchema,
} from '@the-dev-tools/spec/change/v1/change_pb';

import { AutoChangeSource, getBaseMessageMeta, getMessageMeta, registry } from './meta';

interface NormyReactQueryMeta extends Record<string, unknown> {
  normalize?: boolean;
  schema?: DescMethodUnary;
}

declare module '@tanstack/react-query' {
  interface Register {
    queryMeta: NormyReactQueryMeta;
    mutationMeta: NormyReactQueryMeta;
  }
}

const toNormalMessage = (data: unknown, options = { alwaysEmitImplicit: false }) => {
  const { alwaysEmitImplicit } = options;

  if (!isMessage(data)) return Option.none();

  let message: Message | undefined = data;
  if (isMessage(message, AnySchema)) message = anyUnpack(message, registry);
  if (!message) return Option.none();

  const { base, key, normalKeys } = pipe(getBaseMessageMeta(message), Option.getOrNull) ?? {};

  const schema = base ? registry.getMessage(base) : registry.getMessage(message.$typeName);
  if (!schema) return Option.none();

  message = create(schema, message);
  const json = toJson(schema, message, { registry, alwaysEmitImplicit });
  if (!Predicate.isRecord(json)) return Option.none();

  const keys = pipe(Array.fromNullable(key), Array.appendAll(normalKeys ?? []));
  const $id = base ? { $typeName: base, ...Struct.pick(json, ...keys) } : undefined;

  return Option.some({ ...json, $id });
};

const toNormalMessageDeep = (data: unknown): Option.Option<unknown> => {
  if (Array.isArray(data)) return pipe(Array.filterMap(data, toNormalMessageDeep), Option.some);

  if (Predicate.isRecord(data)) {
    const normal = pipe(
      toNormalMessage(data, { alwaysEmitImplicit: true }),
      Option.getOrElse(() => ({})),
    );

    const record = Record.filterMap(data, toNormalMessageDeep);

    return Option.liftPredicate({ ...normal, ...record } as object, (_) => !Record.isEmptyRecord(_));
  }

  return Option.none();
};

const getChanges = (data: unknown): Change[] => {
  if (isMessage(data, ChangeSchema)) return [data];
  if (Array.isArray(data)) return Array.flatMap(data, getChanges);
  if (Predicate.isRecord(data)) return pipe(Record.values(data), getChanges);
  return [];
};

const removeMessageDeep =
  ($id: string) =>
  (data: unknown): Option.Option<unknown> => {
    if (Predicate.hasProperty(data, '$id') && JSON.stringify(data.$id) === $id) return Option.none();

    if (Array.isArray(data)) return pipe(data, Array.map(removeMessageDeep($id)), Array.getSomes, Option.some);

    if (Predicate.isRecord(data)) return pipe(Record.map(data, removeMessageDeep($id)), Record.getSomes, Option.some);

    return Option.some(data);
  };

interface SetQueryDataProps {
  query: Query;
  queryClient: QueryClient;
  queryKey: QueryKey;
  updater: Updater<unknown, unknown>;
}

const setQueryData = ({ query, queryClient, queryKey, updater }: SetQueryDataProps) => {
  // `react-query` resets some state when `setQueryData` is called
  // We reapply state that should not be reset when a query is updated via Normy
  // `dataUpdatedAt` and `isInvalidated` determine if a query is stale or not,
  // and we only want data updates from the network to change it
  const dataUpdatedAt = query.state.dataUpdatedAt;
  const isInvalidated = query.state.isInvalidated;
  const error = query.state.error ?? null;
  const status = query.state.status;

  queryClient.setQueryData(queryKey, updater, {
    updatedAt: dataUpdatedAt,
  });

  query.setState({ isInvalidated, error, status, dataUpdatedAt });
};

interface UpdateQueriesProps {
  data: unknown;
  normalizer: ReturnType<typeof createNormalizer>;
  queryClient: QueryClient;
}

const updateQueries = ({ data, normalizer, queryClient }: UpdateQueriesProps) => {
  pipe(
    normalizer.getQueriesToUpdate(data as Data),
    Array.forEach((query) => {
      const queryKey = JSON.parse(query.queryKey) as QueryKey;
      const cachedQuery = queryClient.getQueryCache().find({ queryKey });

      if (cachedQuery?.queryKey[0] !== 'connect-query') return;

      setQueryData({
        query: cachedQuery,
        queryKey,
        queryClient,
        updater: (data: unknown) =>
          pipe(
            Option.liftPredicate(data, isMessage),
            Option.flatMapNullable((_) => registry.getMessage(_.$typeName)),
            Option.map((_) => fromJson(_, query.data as JsonObject, { registry, ignoreUnknownFields: true })),
            Option.getOrElse(() => data),
          ),
      });
    }),
  );
};

const processChanges = async ({ data, normalizer, queryClient }: UpdateQueriesProps) => {
  const changes = getChanges(data);
  const changesByKind: Record<string, Change[]> = Array.groupBy(changes, (_) => _.kind?.toString() ?? '');

  // Perform deletes
  await pipe(
    changesByKind[ChangeKind.DELETE] ?? [],
    Array.map((_) => toNormalMessage(_.data)),
    Array.getSomes,
    Array.flatMap((data) =>
      pipe(
        normalizer.getDependentQueries(data),
        Array.map((_) => [JSON.stringify(data.$id), JSON.parse(_) as QueryKey] as const),
      ),
    ),
    Array.map(async ([$id, queryKey]) => {
      const query = queryClient.getQueryCache().find({ queryKey });

      if (query?.queryKey[0] !== 'connect-query') return;

      const isTopLevel = pipe(
        toNormalMessage(query.state.data),
        Option.exists((_) => JSON.stringify(_.$id) === $id),
      );

      if (isTopLevel) {
        await queryClient.invalidateQueries({ queryKey, exact: true });
        return;
      }

      setQueryData({
        query,
        queryKey,
        queryClient,
        updater: (data: unknown) => {
          const schema = pipe(
            Option.liftPredicate(data, isMessage),
            Option.flatMapNullable((_) => registry.getMessage(_.$typeName)),
          );

          if (Option.isNone(schema)) return data;

          return pipe(
            toNormalMessageDeep(data),
            Option.flatMap(removeMessageDeep($id)),
            Option.map((_) => fromJson(schema.value, _ as JsonObject, { registry, ignoreUnknownFields: true })),
            Option.getOrElse(() => data),
          );
        },
      });
    }),
    (_) => Promise.allSettled(_),
  );

  // Perform updates
  pipe(
    changesByKind[ChangeKind.UPDATE] ?? [],
    Array.map((_) => toNormalMessage(_.data)),
    Array.getSomes,
    (_) => void updateQueries({ data: _, normalizer, queryClient }),
  );

  // Perform list changes
  pipe(
    Array.flatMap(changes, ({ list, data }) => {
      const messageMaybe = pipe(
        Option.fromNullable(data),
        Option.flatMapNullable((_) => anyUnpack(_, registry)),
        Option.flatMap((_) => toNormalMessage(_, { alwaysEmitImplicit: true })),
      );

      if (Option.isNone(messageMaybe)) return [];

      const messageId = JSON.stringify(messageMaybe.value.$id);
      const message = normalizer.getObjectById<typeof messageMaybe.value>(messageId) ?? messageMaybe.value;

      return list.map((change) => ({ change, message, messageId }));
    }),
    Array.flatMapNullable(({ change, message, messageId }) => {
      const parentMessage = pipe(
        Option.fromNullable(change.parent),
        Option.flatMapNullable((_) => anyUnpack(_, registry)),
      );

      const parentId = pipe(
        Option.flatMap(parentMessage, toNormalMessage),
        Option.map((_) => JSON.stringify(_.$id)),
        Option.getOrElse(() => ''),
      );

      const key = change.key ?? 'items';
      const parent = normalizer.getObjectById(parentId);

      if (!Predicate.isRecord(parent) || !(key in parent) || !Array.isArray(parent[key])) return;

      let list = parent[key] as unknown[];

      if (change.kind === ListChangeKind.APPEND) list = Array.append(list, message);
      if (change.kind === ListChangeKind.PREPEND) list = Array.prepend(list, message);

      if (change.kind === ListChangeKind.REMOVE || change.kind === ListChangeKind.MOVE) {
        list = Array.filter(
          list,
          flow(
            Option.liftPredicate(Predicate.hasProperty('$id')),
            Option.exists((_) => JSON.stringify(_.$id) === messageId),
            Boolean.not,
          ),
        );
      }

      if (
        change.index !== undefined &&
        (change.kind === ListChangeKind.INSERT || change.kind === ListChangeKind.MOVE)
      ) {
        list = pipe(
          Array.insertAt(list, change.index, message),
          Option.getOrElse(() => list),
        );
      }

      return { ...parent, [key]: list };
    }),
    (_) => void updateQueries({ data: _, normalizer, queryClient }),
  );

  // Perform invalidations
  await pipe(
    changesByKind[ChangeKind.INVALIDATE] ?? [],
    Array.map(async (_) => {
      if (!_.service) return;

      const queryKey: ConnectQueryKey = ['connect-query', { serviceName: _.service }];

      if (_.method) queryKey[1].methodName = _.method;

      pipe(
        Option.fromNullable(_.data),
        Option.flatMapNullable((_) => toJson(AnySchema, _, { registry })),
        Option.flatMap(Option.liftPredicate(Predicate.isRecord)),
        Option.map(Struct.omit('@type')),
        Option.map((_) => (queryKey[1].input = _)),
      );

      await queryClient.invalidateQueries({ queryKey });
    }),
    (_) => Promise.allSettled(_),
  );
};

export const createQueryNormalizer = (queryClient: QueryClient) => {
  const normalizer = createNormalizer({
    getNormalizationObjectKey: (data) => {
      if (Predicate.hasProperty(data, '$id')) return JSON.stringify(data.$id);
      return undefined;
    },
  });

  let unsubscribeQueryCache: null | (() => void) = null;
  let unsubscribeMutationCache: null | (() => void) = null;

  return {
    getNormalizedData: normalizer.getNormalizedData,
    setNormalizedData: async (data: unknown) => {
      const message = toNormalMessageDeep(data);
      if (Option.isNone(message)) return;
      await processChanges({ data: message.value, normalizer, queryClient });
    },
    clear: normalizer.clearNormalizedData,
    subscribe: () => {
      unsubscribeQueryCache = queryClient.getQueryCache().subscribe((event) => {
        if (event.type === 'removed') {
          normalizer.removeQuery(JSON.stringify(event.query.queryKey));
        } else if (
          event.type === 'added' &&
          event.query.state.data !== undefined &&
          event.query.meta?.normalize !== false
        ) {
          const message = toNormalMessageDeep(event.query.state.data);
          if (Option.isNone(message)) return;
          normalizer.setQuery(JSON.stringify(event.query.queryKey), message.value as Data);
        } else if (
          event.type === 'updated' &&
          event.action.type === 'success' &&
          event.action.data !== undefined &&
          event.query.meta?.normalize !== false
        ) {
          const message = toNormalMessageDeep(event.action.data);
          if (Option.isNone(message)) return;
          normalizer.setQuery(JSON.stringify(event.query.queryKey), message.value as Data);
        }
      });

      unsubscribeMutationCache = queryClient.getMutationCache().subscribe(async (event) => {
        if (
          event.type === 'updated' &&
          event.action.type === 'success' &&
          event.action.data &&
          event.mutation.meta?.normalize !== false
        ) {
          const input: unknown = event.mutation.state.variables;
          const output: unknown = event.action.data;

          const autoChanges = pipe(
            Option.fromNullable(event.mutation.options.meta?.schema),
            Option.map((method) => {
              const sourceToData = (source?: AutoChangeSource) => {
                const schema = registry.getMessage(source?.$type ?? '');
                if (!source || !schema) return undefined;

                let data = Option.none<Message>();

                if (source.kind !== 'RESPONSE') {
                  data = pipe(
                    Option.liftPredicate(input, Predicate.isRecord),
                    Option.map((_) => create(method.input, _)),
                  );
                }

                if (source.kind !== 'REQUEST') {
                  data = pipe(
                    Option.liftPredicate(output, (_) => isMessage(_, method.output)),
                    Option.map((response) => ({
                      ...Option.getOrElse(data, () => ({})),
                      ...response,
                    })),
                  );
                }

                return pipe(
                  Option.map(data, (_) => create(schema, Struct.omit(_, '$typeName'))),
                  Option.map((_) => anyPack(schema, _)),
                  Option.getOrUndefined,
                );
              };

              const autoChanges = pipe(
                getMessageMeta({ $typeName: method.output.typeName }),
                Option.flatMapNullable((_) => _.autoChanges),
                Option.toArray,
                Array.flatten,
                Array.flatMapNullable(({ $data, $list, ...change }): Change | undefined => {
                  const data = sourceToData($data);

                  const list = Array.flatMapNullable($list ?? [], ({ $parent, ...change }): ListChange | undefined => {
                    const parent = sourceToData($parent);
                    if (!parent) return;
                    return { ...fromJson(ListChangeSchema, change, { registry }), parent };
                  });

                  if (data === undefined) return;
                  return { ...fromJson(ChangeSchema, change, { registry }), data, list };
                }),
              );

              return autoChanges;
            }),
            Option.toArray,
            Array.flatten,
          );

          await processChanges({ data: [autoChanges, output], normalizer, queryClient });
        }
      });
    },
    unsubscribe: () => {
      unsubscribeQueryCache?.();
      unsubscribeMutationCache?.();
      unsubscribeQueryCache = null;
      unsubscribeMutationCache = null;
    },
    getObjectById: normalizer.getObjectById,
    getQueryFragment: normalizer.getQueryFragment,
    getDependentQueries: (mutationData: Data) =>
      normalizer.getDependentQueries(mutationData).map((key) => JSON.parse(key) as QueryKey),
    getDependentQueriesByIds: (ids: readonly string[]) =>
      normalizer.getDependentQueriesByIds(ids).map((key) => JSON.parse(key) as QueryKey),
  };
};

const QueryNormalizerContext = createContext<Option.Option<ReturnType<typeof createQueryNormalizer>>>(Option.none());

export const useQueryNormalizer = () => pipe(useContext(QueryNormalizerContext), Option.getOrThrow);

export const QueryNormalizerProvider = ({
  queryClient,
  children,
}: {
  queryClient: QueryClient;
  children: ReactNode;
}) => {
  const [queryNormalizer] = useState(() => createQueryNormalizer(queryClient));

  useEffect(() => {
    queryNormalizer.subscribe();

    return () => {
      queryNormalizer.unsubscribe();
      queryNormalizer.clear();
    };
  }, [queryNormalizer]);

  return (
    <QueryNormalizerContext.Provider value={Option.some(queryNormalizer)}>{children}</QueryNormalizerContext.Provider>
  );
};
