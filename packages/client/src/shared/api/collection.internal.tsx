import {
  create,
  DescMessage,
  DescMethodServerStreaming,
  DescMethodUnary,
  Message,
  MessageInitShape,
  MessageValidType,
} from '@bufbuild/protobuf';
import { Code, ConnectError, Transport } from '@connectrpc/connect';
import {
  CollectionConfig,
  createCollection,
  createOptimisticAction,
  createPacedMutations,
  debounceStrategy,
  Transaction,
} from '@tanstack/react-db';
import { Array, Effect, HashMap, Match, pipe, Predicate, Record, Struct } from 'effect';
import { Ulid } from 'id128';
import { UnsetSchema } from '@the-dev-tools/spec/buf/global/v1/global_pb';
import { schemas_v1_api } from '@the-dev-tools/spec/tanstack-db/v1/api';
import { request, stream } from './connect-rpc';
import { createAlike, createDelta, draftDelta, mergeDelta, MessageUnion, toUnion, validate } from './protobuf';
import { ApiTransport } from './transport';

export interface ApiCollectionSchema {
  item: DescMessage;
  keys: readonly string[];

  collection: DescMethodUnary;

  sync: {
    method: DescMethodServerStreaming;

    delete: DescMessage;
    insert: DescMessage;
    update: DescMessage;
    upsert: DescMessage;
  };

  operations: {
    delete?: DescMethodUnary;
    insert?: DescMethodUnary;
    update?: DescMethodUnary;
  };
}

export type ApiCollection<TSchema extends ApiCollectionSchema> = ReturnType<typeof createApiCollection<TSchema>>;

const createApiCollection = <TSchema extends ApiCollectionSchema>(schema: TSchema, transport: Transport) => {
  type Item = MessageValidType<TSchema['item']>;
  type ItemKey<T = TSchema['keys'][number]> = T extends keyof Item ? T : never;
  type ItemKeyObject = Pick<Item, ItemKey>;
  type SpecCollectionOptions = CollectionConfig<Item, string>;

  let params: Parameters<SpecCollectionOptions['sync']['sync']>[0];
  let lastSyncTime = 0;

  const getKeyObject = (item: ItemKeyObject) => Struct.pick(item, ...(schema.keys as ItemKey[]));

  const getKey = (item: ItemKeyObject) =>
    pipe(getKeyObject(item) as Record<string, unknown>, (_) =>
      JSON.stringify(_, (key, value: unknown) => {
        if (key.endsWith('Id') && Predicate.isUint8Array(value)) return Ulid.construct(value).toCanonical();
        return value;
      }),
    );

  const parseKeyUnsafe = (key: string) =>
    JSON.parse(key, (key, value: unknown) => {
      if (key.endsWith('Id') && typeof value === 'string') return Ulid.fromCanonical(value).bytes;
      return value;
    }) as ItemKeyObject;

  const sync: SpecCollectionOptions['sync']['sync'] = (_) => {
    params = _;
    const { begin, collection, commit, markReady, write } = params;

    const processSync = (items: Message[]) => {
      begin();
      items.forEach((_) => {
        pipe(
          (_ as Message & { value: MessageUnion }).value,
          (_) => toUnion(_) as Message,
          Match.value,
          Match.when(
            { $typeName: schema.sync.insert.typeName },
            (_: Message) => void write({ type: 'insert', value: createAlike<DescMessage>(schema.item, _) as Item }),
          ),
          Match.when(
            { $typeName: schema.sync.upsert.typeName },
            (_: Message) =>
              void write({
                type: collection.has(getKey(_ as Item)) ? 'update' : 'insert',
                value: createAlike<DescMessage>(schema.item, _) as Item,
              }),
          ),
          Match.when({ $typeName: schema.sync.update.typeName }, (_) => {
            const currentValue = collection.get(getKey(_ as Item));

            if (!currentValue) {
              console.error('Could not apply sync update, as item does not exist in the store', _);
              return;
            }

            write({ type: 'update', value: mergeDelta(currentValue, _, UnsetSchema) });
          }),
          Match.when(
            { $typeName: schema.sync.delete.typeName },
            (_) => void write({ type: 'delete', value: createAlike<DescMessage>(schema.item, _) as Item }),
          ),
          Match.option,
        );
      });
      commit();
      lastSyncTime = Date.now();
    };

    const syncController = new AbortController();

    const sync = async () => {
      const syncStream = stream({
        method: schema.sync.method,
        signal: syncController.signal,
        timeoutMs: 0,
        transport,
      });

      try {
        for await (const response of syncStream) {
          const valid = validate(schema.sync.method.output, response);

          if (valid.kind !== 'valid') {
            console.error('Invalid sync data', valid);
            continue;
          }

          const { items } = valid.message as Message & { items: Message[] };

          if (!initialSyncState.isComplete) {
            initialSyncState.buffer = initialSyncState.buffer.concat(items);
            continue;
          }

          processSync(items);
        }
      } catch (error) {
        if (error instanceof ConnectError && error.code === Code.Canceled) return;
        throw error;
      }
    };

    const initialSyncState = {
      buffer: Array.empty<Message>(),
      isComplete: false,
    };

    const initialSync = async () => {
      const { message } = await request({ method: schema.collection, transport });
      const valid = validate(schema.collection.output, message);

      if (valid.kind !== 'valid') {
        console.error('Invalid initial collection data', valid);
        return;
      }

      begin();
      (valid.message as Message & { items: Item[] }).items.forEach((_) => void write({ type: 'insert', value: _ }));
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

  const makeUtils = () => {
    const waitForSync = (afterTime: number): Promise<void> => {
      if (lastSyncTime > afterTime) return Promise.resolve();

      return new Promise((resolve) => {
        const check = setInterval(() => {
          if (lastSyncTime > afterTime) {
            clearInterval(check);
            resolve();
          }
        }, 100);
      });
    };

    type Operation<Key extends keyof TSchema['operations']> = (
      input: TSchema['operations'][Key] extends DescMethodUnary<infer Input>
        ? MessageInitShape<Input> extends { items?: (infer Item)[] }
          ? Item | Item[]
          : never
        : never,
    ) => Transaction;

    type UpdatePaced = TSchema['operations']['update'] extends DescMethodUnary
      ? { updatePaced: Operation<'update'> }
      : // eslint-disable-next-line @typescript-eslint/no-empty-object-type
        {};

    type Operations = UpdatePaced & {
      [Key in keyof TSchema['operations']]: Operation<Key>;
    };

    const operations = {} as Operations;
    const { delete: delete_, insert, update } = schema.operations;

    if (insert) {
      operations.insert = createOptimisticAction({
        mutationFn: async (input) => {
          const mutationTime = Date.now();
          const items = Array.ensure(input);
          await request({ input: { items }, method: insert, transport });
          await waitForSync(mutationTime);
        },
        onMutate: (input) => {
          pipe(
            Array.ensure(input),
            (_) => create(insert.input, { items: _ }) as Message & { items: Item[] },
            (_) => params.collection.insert(_.items),
          );
        },
      });
    }

    if (update) {
      operations.update = createOptimisticAction({
        mutationFn: async (input) => {
          const mutationTime = Date.now();
          const items = Array.ensure(input);
          await request({ input: { items }, method: update, transport });
          await waitForSync(mutationTime);
        },
        onMutate: (input) => {
          pipe(
            Array.ensure(input),
            (_) => create(update.input, { items: _ }) as Message & { items: Item[] },
            (_) =>
              Array.map(_.items, (delta) => {
                params.collection.update(getKey(delta), (draft: Item) => {
                  draftDelta(draft, delta, UnsetSchema);
                });
              }),
          );
        },
      });

      (operations as { updatePaced: Operation<'update'> }).updatePaced = createPacedMutations({
        mutationFn: async ({ transaction }) => {
          const mutationTime = Date.now();
          const items = transaction.mutations.map((_) =>
            createDelta(update.input.field['items']!.message!, {
              ...parseKeyUnsafe(_.key as string),
              ..._.changes,
            }),
          );
          await request({ input: { items }, method: update, transport });
          await waitForSync(mutationTime);
        },
        onMutate: (input) => {
          pipe(
            Array.ensure(input),
            (_) => create(update.input, { items: _ }) as Message & { items: Item[] },
            (_) =>
              Array.map(_.items, (delta) => {
                params.collection.update(getKey(delta), (draft: Item) => {
                  draftDelta(draft, delta, UnsetSchema);
                });
              }),
          );
        },
        strategy: debounceStrategy({ wait: 200 }),
      });
    }

    if (delete_) {
      operations.delete = createOptimisticAction({
        mutationFn: async (input) => {
          const mutationTime = Date.now();
          const items = Array.ensure(input);
          await request({ input: { items }, method: delete_, transport });
          await waitForSync(mutationTime);
        },
        onMutate: (input) => {
          pipe(
            Array.ensure(input),
            (_) => create(delete_.input, { items: _ }) as Message & { items: Item[] },
            (_) => Array.map(_.items, getKey),
            params.collection.delete,
          );
        },
      });
    }

    return {
      ...operations,
      getKey,
      getKeyObject,
      parseKeyUnsafe,
      state: () => params,
      waitForSync,
    };
  };

  return createCollection({
    gcTime: 0,
    getKey,
    id: schema.item.typeName,
    startSync: true,
    sync: { rowUpdateMode: 'full', sync },
    utils: makeUtils(),
  });
};

export class ApiCollections extends Effect.Service<ApiCollections>()('ApiCollections', {
  effect: Effect.gen(function* () {
    const transport = yield* ApiTransport;

    const collections = pipe(
      Array.map(schemas_v1_api, (schema: ApiCollectionSchema) => {
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
