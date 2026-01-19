import {
  create,
  DescField,
  DescMessage,
  DescMethod,
  DescMethodStreaming,
  fromJson,
  Message,
  MessageInitShape,
  MessageShape,
  ScalarType,
  toJson,
  toJsonString,
} from '@bufbuild/protobuf';
import { timestampFromDate, ValueSchema } from '@bufbuild/protobuf/wkt';
import { createRouterTransport } from '@connectrpc/connect';
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
import {
  HttpResponseAssertSync_ValueUnion_Kind,
  HttpResponseAssertSyncInsertSchema,
  HttpResponseAssertSyncSchema,
  HttpResponseHeaderSync_ValueUnion_Kind,
  HttpResponseHeaderSyncInsertSchema,
  HttpResponseHeaderSyncSchema,
  HttpResponseSync_ValueUnion_Kind,
  HttpResponseSyncInsertSchema,
  HttpResponseSyncSchema,
  HttpRunRequest,
  HttpService,
  HttpSync_ValueUnion_Kind,
  HttpSyncInsertSchema,
  HttpVersionSync_ValueUnion_Kind,
} from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  LogLevel,
  LogService,
  LogSync_ValueUnion_Kind,
  LogSyncResponseSchema,
} from '@the-dev-tools/spec/buf/api/log/v1/log_pb';
import { files } from '@the-dev-tools/spec/buf/files';
import { schemas_v1_api as collections } from '@the-dev-tools/spec/tanstack-db/v1/api';
import { Faker } from '../lib/faker';
import { ApiCollectionSchema } from './collection.internal';
import { effectInterceptor, InterceptorNext, InterceptorRequest } from './connect-rpc';
import { defaultInterceptors } from './interceptors';
import { registry } from './protobuf';

export class UnimplementedMockError extends Data.TaggedError('UnimplementedMockError')<{ reason: string }> {}

const mockScalar = Effect.fn(function* (scalar: ScalarType, field: DescField) {
  const faker = yield* Faker;

  if (scalar === ScalarType.BYTES && field.localName.endsWith('Id')) {
    return Ulid.generate({ time: faker.date.anytime() }).bytes;
  }

  // https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md#scalar-fields
  switch (scalar) {
    case ScalarType.BOOL:
      return faker.datatype.boolean();

    case ScalarType.BYTES:
      return new Uint8Array();

    case ScalarType.DOUBLE:
    case ScalarType.FLOAT:
      return faker.number.float();

    case ScalarType.FIXED32:
    case ScalarType.INT32:
    case ScalarType.SFIXED32:
    case ScalarType.SINT32:
    case ScalarType.UINT32:
      return faker.number.int({ min: 0, max: 2 ** 32 / 2 - 1 });

    case ScalarType.FIXED64:
    case ScalarType.INT64:
    case ScalarType.SFIXED64:
    case ScalarType.SINT64:
    case ScalarType.UINT64:
      return faker.number.bigInt({ min: 0, max: 2n ** 64n / 2n - 1n });

    case ScalarType.STRING:
      return faker.word.words();
  }
});

const mockFieldValue = Effect.fn(function* (field: DescField, depth: number) {
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

const mockField = Effect.fn(function* (field: DescField, depth: number) {
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

const mockMessage = <T extends DescMessage = DescMessage>(
  message: T,
  depth = 0,
): Effect.Effect<MessageShape<T>, UnimplementedMockError, Faker> =>
  Effect.gen(function* () {
    const faker = yield* Faker;

    switch (message.typeName) {
      case 'google.protobuf.Timestamp':
        return timestampFromDate(faker.date.anytime()) as unknown as MessageShape<T>;
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

    return create(message, value as MessageShape<T>);
  });

const mockMethod = Effect.fn(function* (method: DescMethod) {
  const faker = yield* Faker;
  const runtime = yield* Effect.runtime<ApiMockState | Faker>();

  switch (method.methodKind) {
    case 'server_streaming':
      return (input: Message) =>
        Effect.gen(function* () {
          const queue = yield* getStreamQueue(method as DescMethodStreaming, input);

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

const getStreamQueue = Effect.fn(function* <I extends DescMessage = DescMessage, O extends DescMessage = DescMessage>(
  method: DescMethodStreaming<I, O>,
  input?: MessageShape<I>,
) {
  const { streamQueueMap } = yield* ApiMockState;

  let key = method.output.typeName;
  if (input) key += toJsonString(method.input, input);

  let queue = pipe(MutableHashMap.get(streamQueueMap, key), Option.getOrUndefined);

  if (!queue) {
    queue = yield* Queue.unbounded();
    MutableHashMap.set(streamQueueMap, key, queue);
  }

  return queue as Queue.Queue<MessageInitShape<O>>;
});

const mockInterceptor = Effect.fn(function* (next: InterceptorNext, request: InterceptorRequest) {
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
    methodImplMap: MutableHashMap.empty<DescMethod>(),
    streamQueueMap: MutableHashMap.empty<string, Queue.Queue<unknown>>(),
  }),
}) {}

export class ApiTransportMock extends Effect.Service<ApiTransportMock>()('ApiTransportMock', {
  dependencies: [ApiMockState.Default, Faker.Default],
  effect: Effect.gen(function* () {
    yield* mockCollections;
    yield* mockHttpRun;

    const { methodImplMap } = yield* ApiMockState;

    const methods = pipe(
      Array.flatMap(files, (_) => _.services),
      Array.flatMap((_) => _.methods),
    );

    for (const method of methods) {
      if (pipe(MutableHashMap.get(methodImplMap, method), Option.isSome)) continue;
      MutableHashMap.set(methodImplMap, method, yield* mockMethod(method));
    }

    return createRouterTransport(
      (router) => {
        methods.forEach((method) => {
          const impl = pipe(MutableHashMap.get(methodImplMap, method), Option.getOrThrow);
          router.rpc(method, impl, {});
        });
      },
      {
        transport: {
          interceptors: [yield* effectInterceptor(mockInterceptor), ...defaultInterceptors],
        },
      },
    );
  }),
}) {}

const mockCollections = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<ApiMockState>();
  const { methodImplMap } = yield* ApiMockState;

  for (const collection of collections as ApiCollectionSchema[]) {
    const syncQueue = yield* getStreamQueue(collection.sync.method);

    const syncImpl = () => pipe(syncQueue, Stream.fromQueue, Stream.toAsyncIterable);
    MutableHashMap.set(methodImplMap, collection.sync.method, syncImpl);

    const { delete: delete_, insert, update } = collection.operations;

    const syncUnion = registry.getMessage(`${collection.item.typeName}Sync.ValueUnion`)!;

    const toSyncOutput = Effect.fn(function* (input: Message, operation: string, logLevel?: LogLevel) {
      yield* mockLog(input, logLevel);

      const items = (input as Message & { items: Message[] }).items.map((item) => ({
        value: {
          kind: syncUnion.field[operation]!.number,
          [operation]: item,
        },
      }));

      const sync = create(collection.sync.method.output, { items });

      yield* Queue.offer(syncQueue, sync);
    });

    MutableHashMap.set(methodImplMap, collection.collection, () => ({}));

    if (insert) {
      const insertImpl = (input: Message) =>
        toSyncOutput(input, 'insert', LogLevel.WARNING).pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, insert, insertImpl);
    }

    if (update) {
      const updateImpl = (input: Message) => toSyncOutput(input, 'update').pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, update, updateImpl);
    }

    if (delete_) {
      const deleteImpl = (input: Message) =>
        toSyncOutput(input, 'delete', LogLevel.ERROR).pipe(Runtime.runPromise(runtime));
      MutableHashMap.set(methodImplMap, delete_, deleteImpl);
    }
  }
});

const mockLog = Effect.fn(function* (message: Message, level: LogLevel = LogLevel.UNSPECIFIED) {
  const value = pipe(
    registry.getMessage(message.$typeName)!,
    (_) => toJson(_, message),
    (_) => fromJson(ValueSchema, _, { registry }),
  );

  const queue = yield* getStreamQueue(LogService.method.logSync);

  const sync = create(LogSyncResponseSchema, {
    items: [
      {
        value: {
          insert: { level, logId: Ulid.generate().bytes, name: message.$typeName, value },
          kind: LogSync_ValueUnion_Kind.INSERT,
        },
      },
    ],
  });

  yield* Queue.offer(queue, sync);
});

const mockHttpRun = Effect.gen(function* () {
  const faker = yield* Faker;
  const runtime = yield* Effect.runtime<Faker>();
  const { methodImplMap } = yield* ApiMockState;

  const httpQueue = yield* getStreamQueue(HttpService.method.httpSync);
  const versionQueue = yield* getStreamQueue(HttpService.method.httpVersionSync);
  const responseQueue = yield* getStreamQueue(HttpService.method.httpResponseSync);
  const headerQueue = yield* getStreamQueue(HttpService.method.httpResponseHeaderSync);
  const assertQueue = yield* getStreamQueue(HttpService.method.httpResponseAssertSync);

  const impl = ({ httpId }: HttpRunRequest) =>
    Effect.gen(function* () {
      const sendResponse = Effect.fn(function* (httpId: Uint8Array) {
        const httpResponseId = Ulid.generate().bytes;

        const response: MessageInitShape<typeof HttpResponseSyncSchema> = {
          value: {
            insert: { ...(yield* mockMessage(HttpResponseSyncInsertSchema)), httpId, httpResponseId },
            kind: HttpResponseSync_ValueUnion_Kind.INSERT,
          },
        };

        const headers = yield* pipe(
          faker.number.int({ min: 3, max: 10 }),
          Array.makeBy(() =>
            pipe(
              mockMessage(HttpResponseHeaderSyncInsertSchema),
              Effect.map(
                (_): MessageInitShape<typeof HttpResponseHeaderSyncSchema> => ({
                  value: {
                    insert: { ..._, httpResponseId },
                    kind: HttpResponseHeaderSync_ValueUnion_Kind.INSERT,
                  },
                }),
              ),
            ),
          ),
          Effect.all,
        );

        const asserts = yield* pipe(
          faker.number.int({ min: 3, max: 10 }),
          Array.makeBy(() =>
            pipe(
              mockMessage(HttpResponseAssertSyncInsertSchema),
              Effect.map(
                (_): MessageInitShape<typeof HttpResponseAssertSyncSchema> => ({
                  value: {
                    insert: { ..._, httpResponseId },
                    kind: HttpResponseAssertSync_ValueUnion_Kind.INSERT,
                  },
                }),
              ),
            ),
          ),
          Effect.all,
        );

        yield* Queue.offer(headerQueue, { items: headers });
        yield* Queue.offer(assertQueue, { items: asserts });
        yield* Queue.offer(responseQueue, { items: [response] });
      });

      const version = {
        ...(yield* mockMessage(HttpSyncInsertSchema)),
        httpId: Ulid.generate().bytes,
      };

      yield* Queue.offer(httpQueue, {
        items: [{ value: { insert: version, kind: HttpSync_ValueUnion_Kind.INSERT } }],
      });

      yield* Queue.offer(versionQueue, {
        items: [
          {
            value: {
              insert: { httpId, httpVersionId: version.httpId },
              kind: HttpVersionSync_ValueUnion_Kind.INSERT,
            },
          },
        ],
      });

      yield* sendResponse(httpId);
      yield* sendResponse(version.httpId);
    }).pipe(Runtime.runPromise(runtime));

  MutableHashMap.set(methodImplMap, HttpService.method.httpRun, impl);
});
