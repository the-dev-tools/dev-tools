import {
  create,
  DescEnum,
  DescField,
  DescMessage,
  DescService,
  Message,
  ScalarType,
  toJsonString,
} from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createRouterTransport, Interceptor, ServiceImpl } from '@connectrpc/connect';
import { Faker as FakerClass, base as fakerLocaleBase, en as fakerLocaleEn } from '@faker-js/faker';
import { Effect, MutableHashMap, Option, pipe, Record } from 'effect';
import { Ulid } from 'id128';
import { files } from '@the-dev-tools/spec/files';
import { NodeKind, NodeListResponseSchema } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { defaultInterceptors } from './interceptors';

class Faker extends Effect.Service<Faker>()('Faker', {
  sync: () => {
    const faker = new FakerClass({ locale: [fakerLocaleEn, fakerLocaleBase] });
    faker.seed(0);
    return faker;
  },
}) {}

const fakeScalar = (faker: (typeof Faker)['Service'], scalar: ScalarType, field: DescField) => {
  if (field.name.endsWith('Id')) {
    const id = Ulid.generate({ time: faker.date.anytime() });
    return id.bytes;
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
      return faker.number.int({ max: 2 ** 32 / 2 - 1, min: 0 });

    case ScalarType.FIXED64:
    case ScalarType.INT64:
    case ScalarType.SFIXED64:
    case ScalarType.SINT64:
    case ScalarType.UINT64:
      return faker.number.bigInt({ max: 2n ** 64n / 2n - 1n, min: 0 });

    case ScalarType.STRING:
      return faker.word.words();
  }
};

const fakeEnum = (faker: (typeof Faker)['Service'], enum_: DescEnum) =>
  faker.number.int({
    max: enum_.values.length - 1,
    min: 1,
  });

const fakeMessage = (faker: (typeof Faker)['Service'], message: DescMessage, depth = 0): Message => {
  switch (message.typeName) {
    case 'flow.edge.v1.EdgeListResponse':
      return create(message);

    case 'flow.node.v1.NodeListResponse':
      return create(NodeListResponseSchema, {
        items: [
          {
            kind: NodeKind.NO_OP,
            nodeId: new Uint8Array(),
            position: { x: 0, y: 0 },
          },
        ],
      });

    case 'google.protobuf.Timestamp':
      return timestampFromDate(faker.date.anytime());
  }

  const value = Record.map(message.field, (field) => {
    switch (field.fieldKind) {
      case 'enum':
        return fakeEnum(faker, field.enum);

      case 'list':
        if (field.name === 'changes' && field.message?.typeName === 'change.v1.Change') return [];
        if (depth > 5) return [];
        return faker.helpers.multiple(() => {
          switch (field.listKind) {
            case 'enum':
              return fakeEnum(faker, field.enum);

            case 'message':
              return fakeMessage(faker, field.message, depth + 1);

            case 'scalar':
              return fakeScalar(faker, field.scalar, field);
          }
        });

      case 'message':
        if (depth > 5) return undefined;
        return fakeMessage(faker, field.message, depth + 1);

      case 'scalar':
        return fakeScalar(faker, field.scalar, field);

      default:
        throw new Error('Unimplemented field kind');
    }
  });

  return create(message, value);
};

const cache = MutableHashMap.empty<string, Message>();
const streamCache = MutableHashMap.empty<string, AsyncIterable<Message>>();

const mockInterceptor: Interceptor = (next) => async (request) => {
  const response = await next(request);
  console.log(`Mocking ${request.url}`, { request, response });
  await new Promise((_) => setTimeout(_, 500));
  return response;
};

const mockServiceMethods = (faker: Faker, service: DescService): ServiceImpl<never> =>
  Record.map(service.method, (method) => {
    const makeKey = (input: Message) => method.input.typeName + toJsonString(method.input, input);
    const makeMessage = () => fakeMessage(faker, method.output);

    switch (method.methodKind) {
      case 'server_streaming':
        return (input: Message) => {
          const key = makeKey(input);

          const stream = pipe(
            MutableHashMap.get(streamCache, key),
            Option.getOrElse(() =>
              (async function* () {
                // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
                while (true) {
                  await new Promise((_) => setTimeout(_, 2000));
                  yield makeMessage();
                }
              })(),
            ),
          );

          MutableHashMap.set(streamCache, key, stream);
          return stream;
        };

      case 'unary':
        return (input: Message) => {
          const key = makeKey(input);
          const message = pipe(MutableHashMap.get(cache, key), Option.getOrElse(makeMessage));
          MutableHashMap.set(cache, key, message);
          return message;
        };

      default:
        throw new Error('Unimplemented method kind');
    }
  });

export class ApiTransportMock extends Effect.Service<ApiTransportMock>()('ApiTransportMock', {
  dependencies: [Faker.Default],
  effect: Effect.gen(function* () {
    const faker = yield* Faker;
    return createRouterTransport(
      (router) => {
        files.forEach((file) => {
          file.services.forEach((service) => {
            const methods = mockServiceMethods(faker, service);
            router.service(service, methods);
          });
        });
      },
      {
        transport: {
          interceptors: [mockInterceptor, ...defaultInterceptors],
        },
      },
    );
  }),
}) {}
