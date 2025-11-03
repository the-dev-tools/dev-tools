import { CollectionConfig, createCollection } from '@tanstack/react-db';
import { Array, Effect, HashMap, Match, pipe, Predicate, Record, Runtime } from 'effect';
import { Ulid } from 'id128';
import { UnsetSchema } from '@the-dev-tools/spec/global/v1/global_pb';
import { schemas_v1_api } from '@the-dev-tools/spec/tanstack-db/v1/api';
import { ApiTransport } from '~/api/transport';
import { rootRouteApi } from '~/routes';
import * as Connect from './connect-rpc';
import * as Protobuf from './protobuf';

interface CollectionSchema<T extends Protobuf.DescMessage = Protobuf.DescMessage> {
  item: T;
  keys: readonly string[];

  collection: Protobuf.DescMethodUnary;

  sync: {
    method: Protobuf.DescMethodServerStreaming;

    delete: Protobuf.DescMessage;
    insert: Protobuf.DescMessage;
    update: Protobuf.DescMessage;
  };
}

const createApiCollection = <T extends Protobuf.DescMessage>(
  schema: CollectionSchema<T>,
  transport: Connect.Transport,
) => {
  type Item = Protobuf.MessageValidType<T>;
  type SpecCollectionOptions = CollectionConfig<Item, string>;

  const getKey: SpecCollectionOptions['getKey'] = (item) =>
    pipe(
      Record.fromIterableWith(schema.keys, (_) => [_, item[_ as keyof Item]]),
      Record.map((_: unknown, key) => {
        if (key.includes('Id') && Predicate.isUint8Array(_)) return Ulid.construct(_).toCanonical();
        return _;
      }),
      JSON.stringify,
    );

  const sync: SpecCollectionOptions['sync']['sync'] = (params) => {
    const { begin, collection, commit, markReady, write } = params;

    const processSync = (items: Protobuf.Message[]) => {
      begin();
      items.forEach((_) => {
        pipe(
          _ as Protobuf.MessageUnion,
          (_) => Protobuf.toUnion(_) as Protobuf.Message,
          Match.value,
          Match.when(
            { $typeName: schema.sync.insert.typeName },
            (_: Protobuf.Message) =>
              void write({ type: 'insert', value: Protobuf.createAlike<Protobuf.DescMessage>(schema.item, _) as Item }),
          ),
          Match.when(
            { $typeName: schema.sync.delete.typeName },
            (_) =>
              void write({ type: 'delete', value: Protobuf.createAlike<Protobuf.DescMessage>(schema.item, _) as Item }),
          ),
          Match.when({ $typeName: schema.sync.update.typeName }, (_) => {
            const currentValue = collection.get(getKey(_ as Item));

            if (!currentValue) {
              console.error('Could not apply sync update, as item does not exist in the store', _);
              return;
            }

            write({ type: 'update', value: Protobuf.mergeDelta(currentValue, _, UnsetSchema) });
          }),
          Match.option,
        );
      });
      commit();
    };

    const syncController = new AbortController();

    const sync = async () => {
      const stream = await Connect.stream({ method: schema.sync.method, signal: syncController.signal, transport });

      for await (const response of stream.message) {
        const valid = Protobuf.validate(schema.sync.method.output, response);

        if (valid.kind !== 'valid') {
          console.error('Invalid sync data', valid);
          continue;
        }

        const { items } = valid.message as Protobuf.Message & { items: Protobuf.Message[] };

        if (!initialSyncState.isComplete) {
          initialSyncState.buffer = initialSyncState.buffer.concat(items);
          continue;
        }

        processSync(items);
      }
    };

    const initialSyncState = {
      buffer: Array.empty<Protobuf.Message>(),
      isComplete: false,
    };

    const initialSync = async () => {
      const { message } = await Connect.request({ method: schema.collection, transport });
      const valid = Protobuf.validate(schema.collection.output, message);

      if (valid.kind !== 'valid') {
        console.error('Invalid initial collection data', valid);
        return;
      }

      begin();
      (valid.message as Protobuf.Message & { items: Item[] }).items.forEach(
        (_) => void write({ type: 'insert', value: _ }),
      );
      commit();

      initialSyncState.isComplete = true;

      if (initialSyncState.buffer.length > 0) processSync(initialSyncState.buffer);

      markReady();
    };

    void sync();
    void initialSync();

    return () => {
      syncController.abort();
    };
  };

  return createCollection({
    gcTime: Infinity,
    getKey,
    id: schema.item.typeName,
    startSync: true,
    sync: { rowUpdateMode: 'full', sync },
  });
};

export class ApiCollections extends Effect.Service<ApiCollections>()('ApiCollections', {
  effect: Effect.gen(function* () {
    const transport = yield* ApiTransport;

    const collections = pipe(
      Array.map(schemas_v1_api, (schema: CollectionSchema) => {
        const collection = createApiCollection(schema, transport);
        return [schema, collection] as const;
      }),
      HashMap.fromIterable,
    );

    yield* pipe(
      HashMap.toValues(collections),
      Array.map((_) => Effect.tryPromise(() => _.waitFor('status:ready'))),
      (_) => Effect.all(_, { concurrency: 'unbounded' }),
    );

    return collections;
  }),
}) {}

export const getApiCollection = Effect.fn(function* <T extends Protobuf.DescMessage>(schema: CollectionSchema<T>) {
  const collectionMap = yield* ApiCollections;
  const collection = yield* HashMap.get(collectionMap, schema);
  return collection as unknown as ReturnType<typeof createApiCollection<T>>;
});

export const useApiCollection = <T extends Protobuf.DescMessage>(schema: CollectionSchema<T>) => {
  const { runtime } = rootRouteApi.useRouteContext();
  return Runtime.runSync(runtime, getApiCollection(schema));
};
