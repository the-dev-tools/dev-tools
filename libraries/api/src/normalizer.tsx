import { create, DescMethodUnary, fromJson, isMessage, JsonObject, Message, toJson } from '@bufbuild/protobuf';
import { AnySchema, anyUnpack } from '@bufbuild/protobuf/wkt';
import { ConnectQueryKey } from '@connectrpc/connect-query';
import { createNormalizer, Data } from '@normy/core';
import { QueryClient, QueryKey } from '@tanstack/react-query';
import { Array, Option, pipe, Predicate, Record, Struct } from 'effect';
import { createContext, ReactNode, useContext, useEffect, useState } from 'react';

import { Change, ChangeKind, ChangeSchema } from '@the-dev-tools/spec/change/v1/change_pb';

import { getMessageMeta, registry } from './meta';

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

const toNormalMessage = (data: unknown) => {
  if (!isMessage(data)) return Option.none();

  let message: Message | undefined = data;
  if (isMessage(message, AnySchema)) message = anyUnpack(message, registry);
  if (!message) return Option.none();

  const schema = registry.getMessage(message.$typeName);
  if (!schema) return Option.none();

  const json = toJson(schema, message, { registry });
  if (!Predicate.isRecord(json)) return Option.none();

  const { base, key, normalKeys } = pipe(getMessageMeta(message), Option.getOrNull) ?? {};
  if (!base) return Option.none();

  const keys = pipe(Array.fromNullable(key), Array.appendAll(normalKeys ?? []));
  const $id = { $typeName: base, ...Struct.pick(json, ...keys) };

  return Option.some({ ...json, $id });
};

const toNormalMessageDeep = (data: unknown): Option.Option<unknown> => {
  if (Array.isArray(data)) return pipe(Array.filterMap(data, toNormalMessageDeep), Option.some);

  if (Predicate.isRecord(data)) {
    const normal = pipe(
      toNormalMessage(data),
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

const updateQueriesFromMutationData = async (
  mutationData: Data,
  normalizer: ReturnType<typeof createNormalizer>,
  queryClient: QueryClient,
) => {
  const changesByKind: Record<string, Change[]> = pipe(
    getChanges(mutationData),
    Array.groupBy((_) => _.kind.toString()),
  );

  // Perform updates
  pipe(
    changesByKind[ChangeKind.UPDATE.toString()] ?? [],
    Array.map((_) => toNormalMessage(_.data)),
    Array.getSomes,
    normalizer.getQueriesToUpdate,
    Array.forEach((query) => {
      const queryKey = JSON.parse(query.queryKey) as QueryKey;
      const cachedQuery = queryClient.getQueryCache().find({ queryKey });

      if (cachedQuery?.queryKey[0] !== 'connect-query' || !Predicate.isRecord(query.data)) return;

      const [_, { serviceName, methodName }] = cachedQuery.queryKey as ConnectQueryKey;
      const methodKey = `${methodName?.[0]?.toLowerCase()}${methodName?.slice(1)}`;
      const method = registry.getService(serviceName)?.method[methodKey];

      if (!method) return;

      // `react-query` resets some state when `setQueryData` is called
      // We reapply state that should not be reset when a query is updated via Normy
      // `dataUpdatedAt` and `isInvalidated` determine if a query is stale or not,
      // and we only want data updates from the network to change it
      const dataUpdatedAt = cachedQuery.state.dataUpdatedAt;
      const isInvalidated = cachedQuery.state.isInvalidated;
      const error = cachedQuery.state.error ?? null;
      const status = cachedQuery.state.status;

      const message = fromJson(method.output, query.data as JsonObject, { ignoreUnknownFields: true });

      queryClient.setQueryData(queryKey, () => message, {
        updatedAt: dataUpdatedAt,
      });

      cachedQuery.setState({ isInvalidated, error, status, dataUpdatedAt });
    }),
  );

  // Perform invalidations
  await pipe(
    changesByKind[ChangeKind.INVALIDATE.toString()] ?? [],
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
    setNormalizedData: (data: Data) => updateQueriesFromMutationData(data, normalizer, queryClient),
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
          const data: unknown[] = [event.action.data];
          if (event.mutation.options.meta?.schema && Predicate.isRecord(event.mutation.state.variables))
            data.push(create(event.mutation.options.meta.schema.input, event.mutation.state.variables));
          await updateQueriesFromMutationData(data as Data, normalizer, queryClient);
        } else if (
          event.type === 'updated' &&
          event.action.type === 'pending' &&
          // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
          (event.mutation.state?.context as { optimisticData?: Data })?.optimisticData
        ) {
          const data: unknown[] = [(event.mutation.state.context as { optimisticData: Data }).optimisticData];
          if (event.mutation.options.meta?.schema && Predicate.isRecord(event.mutation.state.variables))
            data.push(create(event.mutation.options.meta.schema.input, event.mutation.state.variables));
          await updateQueriesFromMutationData(data as Data, normalizer, queryClient);
        } else if (
          event.type === 'updated' &&
          event.action.type === 'error' &&
          // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
          (event.mutation.state?.context as { rollbackData?: Data })?.rollbackData
        ) {
          const data: unknown[] = [(event.mutation.state.context as { rollbackData: Data }).rollbackData];
          if (event.mutation.options.meta?.schema && Predicate.isRecord(event.mutation.state.variables))
            data.push(create(event.mutation.options.meta.schema.input, event.mutation.state.variables));
          await updateQueriesFromMutationData(data as Data, normalizer, queryClient);
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
