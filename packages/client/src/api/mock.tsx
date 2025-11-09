import {
  Array,
  Cause,
  Data,
  Duration,
  Effect,
  MutableHashMap,
  Option,
  pipe,
  Queue,
  Record,
  Runtime,
  Stream,
} from 'effect';
import { Ulid } from 'id128';
import { files } from '@the-dev-tools/spec/files';
import { schemas_v1_api as collections } from '@the-dev-tools/spec/tanstack-db/v1/api';
import { ApiCollectionSchema, Connect, Protobuf } from '~/api-new';
import { Faker } from '~/utils/faker';
import { registry } from '~api-new/protobuf';
import { defaultInterceptors } from './interceptors';

export class UnimplementedMockError extends Data.TaggedError('UnimplementedMockError')<{ reason: string }> {}

const mockScalar = Effect.fn(function* (scalar: Protobuf.ScalarType, field: Protobuf.DescField) {
  const faker = yield* Faker;

  if (scalar === Protobuf.ScalarType.BYTES && field.localName.endsWith('Id')) {
    return Ulid.generate({ time: faker.date.anytime() }).bytes;
  }

  // https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md#scalar-fields
  switch (scalar) {
    case Protobuf.ScalarType.BOOL:
      return faker.datatype.boolean();

    case Protobuf.ScalarType.BYTES:
      return new Uint8Array();

    case Protobuf.ScalarType.DOUBLE:
    case Protobuf.ScalarType.FLOAT:
      return faker.number.float();

    case Protobuf.ScalarType.FIXED32:
    case Protobuf.ScalarType.INT32:
    case Protobuf.ScalarType.SFIXED32:
    case Protobuf.ScalarType.SINT32:
    case Protobuf.ScalarType.UINT32:
      return faker.number.int({ min: 0, max: 2 ** 32 / 2 - 1 });

    case Protobuf.ScalarType.FIXED64:
    case Protobuf.ScalarType.INT64:
    case Protobuf.ScalarType.SFIXED64:
    case Protobuf.ScalarType.SINT64:
    case Protobuf.ScalarType.UINT64:
      return faker.number.bigInt({ min: 0, max: 2n ** 64n / 2n - 1n });

    case Protobuf.ScalarType.STRING:
      return faker.word.words();
  }
});

const mockFieldValue = Effect.fn(function* (field: Protobuf.DescField, depth: number) {
  const faker = yield* Faker;

  /* eslint-disable @typescript-eslint/no-unnecessary-condition */
  if (
    field.fieldKind === 'enum' ||
    (field.fieldKind === 'list' && field.listKind === 'enum') ||
    (field.fieldKind === 'map' && field.mapKind === 'enum')
  ) {
    return faker.helpers.arrayElement(field.enum.values).number;
  }

  if (
    field.fieldKind === 'message' ||
    (field.fieldKind === 'list' && field.listKind === 'message') ||
    (field.fieldKind === 'map' && field.mapKind === 'message')
  ) {
    return yield* mockMessage(field.message, depth + 1);
  }

  if (
    field.fieldKind === 'scalar' ||
    (field.fieldKind === 'list' && field.listKind === 'scalar') ||
    (field.fieldKind === 'map' && field.mapKind === 'scalar')
  ) {
    return yield* mockScalar(field.scalar, field);
  }

  return yield* new UnimplementedMockError({ reason: 'Unimplemented field kind' });
  /* eslint-enable @typescript-eslint/no-unnecessary-condition */
});

const mockField = Effect.fn(function* (field: Protobuf.DescField, depth: number) {
  const faker = yield* Faker;

  let value: unknown;

  switch (field.fieldKind) {
    case 'list': {
      const list = Array.empty<unknown>();
      value = list;
      if (depth > 5) break;
      for (let index = 0; index < faker.number.int({ min: 3, max: 10 }); index++)
        list.push(yield* mockFieldValue(field, depth));
      break;
    }

    case 'map': {
      const map = Record.empty<string, unknown>();
      value = map;
      if (depth > 5) break;
      const length = faker.number.int({ min: 3, max: 10 });
      const keys = faker.helpers.uniqueArray(() => faker.word.sample(), length);
      for (const key of keys) map[key] = yield* mockFieldValue(field, depth);
      break;
    }

    default:
      value = yield* mockFieldValue(field, depth);
  }

  return value;
});

const mockMessage = <T extends Protobuf.DescMessage = Protobuf.DescMessage>(
  message: T,
  depth = 0,
): Effect.Effect<Protobuf.MessageShape<T>, UnimplementedMockError, Faker> =>
  Effect.gen(function* () {
    const faker = yield* Faker;

    switch (message.typeName) {
      case 'google.protobuf.Timestamp':
        return Protobuf.WKT.timestampFromDate(faker.date.anytime()) as unknown as Protobuf.MessageShape<T>;
    }

    const value: Record<string, unknown> = {};

    for (const member of message.members) {
      if (member.kind === 'field') {
        value[member.localName] = yield* mockField(member, depth);
      } else {
        const field = faker.helpers.arrayElement(member.fields);
        value[member.localName] = {
          case: field.localName,
          value: yield* mockField(field, depth),
        };
      }
    }

    return Protobuf.create(message, value as Protobuf.MessageShape<T>);
  });

const mockMethod = Effect.fn(function* (method: Protobuf.DescMethod) {
  const faker = yield* Faker;
  const runtime = yield* Effect.runtime<ApiMockState | Faker>();

  switch (method.methodKind) {
    case 'server_streaming':
      return (input: Protobuf.Message) =>
        Effect.gen(function* () {
          const queue = yield* getStreamQueue(method as Protobuf.DescMethodStreaming, input);

          for (let index = 0; index < faker.number.int({ min: 3, max: 10 }); index++) {
            const message = yield* mockMessage(method.output);
            yield* Queue.offer(queue, message);
          }

          return pipe(Stream.fromQueue(queue), Stream.toAsyncIterable);
        }).pipe(Runtime.runSync(runtime));

    case 'unary':
      return () => pipe(mockMessage(method.output), Runtime.runSync(runtime));

    default:
      return yield* new UnimplementedMockError({ reason: 'Unimplemented method kind' });
  }
});

const getStreamQueue = Effect.fn(function* <
  I extends Protobuf.DescMessage = Protobuf.DescMessage,
  O extends Protobuf.DescMessage = Protobuf.DescMessage,
>(method: Protobuf.DescMethodStreaming<I, O>, input?: Protobuf.MessageShape<I>) {
  const { streamQueueMap } = yield* ApiMockState;

  let key = method.output.typeName;
  if (input) key += Protobuf.toJsonString(method.input, input);

  let queue = pipe(MutableHashMap.get(streamQueueMap, key), Option.getOrUndefined);

  if (!queue) {
    queue = yield* Queue.unbounded();
    MutableHashMap.set(streamQueueMap, key, queue);
  }

  return queue as Queue.Queue<Protobuf.MessageInitShape<O>>;
});

const mockInterceptor = Effect.fn(function* (next: Connect.InterceptorNext, request: Connect.InterceptorRequest) {
  const { name } = request.method;

  const delay = Duration.decode('500 millis');

  yield* Effect.annotateLogsScoped({ delay: Duration.format(delay), request });

  if (request.stream) yield* Effect.logDebug(`Mock stream init ${name}`);

  const response = yield* Effect.tryPromise(() => next(request));

  yield* Effect.annotateLogsScoped({ response });

  if (response.stream) {
    const message = yield* pipe(
      Stream.fromAsyncIterable(response.message, (_) => new Cause.UnknownException(_)),
      Stream.tap(
        Effect.fn(function* (message) {
          yield* Effect.annotateLogsScoped({ message });
          yield* Effect.logDebug(`Mock stream message ${name}`);
          yield* Effect.sleep(delay);
        }, Effect.scoped),
      ),
      Stream.toAsyncIterableEffect,
    );

    return { ...response, message };
  } else {
    yield* Effect.logDebug(`Mock request ${name}`);
    yield* Effect.sleep(delay);
    return response;
  }
}, Effect.scoped);

class ApiMockState extends Effect.Service<ApiMockState>()('ApiMockState', {
  sync: () => ({
    methodImplMap: MutableHashMap.empty<Protobuf.DescMethod>(),
    streamQueueMap: MutableHashMap.empty<string, Queue.Queue<unknown>>(),
  }),
}) {}

export class ApiTransportMock extends Effect.Service<ApiTransportMock>()('ApiTransportMock', {
  dependencies: [ApiMockState.Default, Faker.Default],
  effect: Effect.gen(function* () {
    yield* mockCollections;

    const { methodImplMap } = yield* ApiMockState;

    const methods = pipe(
      Array.flatMap(files, (_) => _.services),
      Array.flatMap((_) => _.methods),
    );

    for (const method of methods) {
      if (pipe(MutableHashMap.get(methodImplMap, method), Option.isSome)) continue;
      MutableHashMap.set(methodImplMap, method, yield* mockMethod(method));
    }

    return Connect.createRouterTransport(
      (router) => {
        methods.forEach((method) => {
          const impl = pipe(MutableHashMap.get(methodImplMap, method), Option.getOrThrow);
          router.rpc(method, impl, {});
        });
      },
      {
        transport: {
          interceptors: [yield* Connect.effectInterceptor(mockInterceptor), ...defaultInterceptors],
        },
      },
    );
  }),
}) {}

const mockCollections = Effect.gen(function* () {
  const runtime = yield* Effect.runtime();
  const { methodImplMap } = yield* ApiMockState;

  for (const collection of collections as ApiCollectionSchema[]) {
    const syncQueue = yield* getStreamQueue(collection.sync.method);

    const syncImpl = () => pipe(syncQueue, Stream.fromQueue, Stream.toAsyncIterable);
    MutableHashMap.set(methodImplMap, collection.sync.method, syncImpl);

    const { delete: delete_, insert, update } = collection.operations;

    const syncUnion = registry.getMessage(`${collection.item.typeName}Sync.ValueUnion`)!;

    const toSyncOutput = Effect.fn(function* (input: Protobuf.Message, operation: string) {
      const items = (input as Protobuf.Message & { items: Protobuf.Message[] }).items.map((item) => ({
        value: {
          kind: syncUnion.field[operation]!.number,
          [operation]: item,
        },
      }));

      const sync = Protobuf.create(collection.sync.method.output, { items });

      yield* Queue.offer(syncQueue, sync);
    });

    if (insert) {
      const insertImpl = (input: Protobuf.Message) => toSyncOutput(input, 'insert').pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, insert, insertImpl);
    }

    if (update) {
      const updateImpl = (input: Protobuf.Message) => toSyncOutput(input, 'update').pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, update, updateImpl);
    }

    if (delete_) {
      const deleteImpl = (input: Protobuf.Message) => toSyncOutput(input, 'delete').pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, delete_, deleteImpl);
    }
  }
});
