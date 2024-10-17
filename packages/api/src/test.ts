import { create, DescEnum, DescField, DescMessage, Message, ScalarType } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createRouterTransport, ServiceImpl } from '@connectrpc/connect';
import { Schema } from '@effect/schema';
import { Context, DateTime, Effect, flow, Layer, pipe, Record, Ref, Runtime } from 'effect';
import { Ulid } from 'id128';
import { UnsecuredJWT } from 'jose';
import { Magic, PromiEvent } from 'magic-sdk';

import {
  AuthMagicLinkResponseSchema,
  AuthRefreshResponseSchema,
  AuthService,
} from '@the-dev-tools/spec/auth/v1/auth_pb';
import { files } from '@the-dev-tools/spec/files';
import { Faker, FakerLive } from '@the-dev-tools/utils/faker';

import { authorizationInterceptor, AuthTransport, MagicClient } from './auth';
import { AccessTokenPayload, RefreshTokenPayload } from './jwt';
import { AnyFnEffect, ApiTransport, effectInterceptor, Request } from './transport';

class EmailRef extends Context.Tag('EmailRef')<EmailRef, Ref.Ref<string>>() {}

const EmailTest = Layer.effect(
  EmailRef,
  Effect.flatMap(Faker, (_) => Ref.make(_.internet.email())),
);

const AuthTransportTest = Layer.effect(
  AuthTransport,
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<Faker | EmailRef>();
    return createRouterTransport(
      (router) => {
        router.service(AuthService, {
          authMagicLink: () =>
            pipe(
              tokens,
              Effect.map((_) => create(AuthMagicLinkResponseSchema, _)),
              Runtime.runPromise(runtime),
            ),
          authRefresh: () =>
            pipe(
              tokens,
              Effect.map((_) => create(AuthRefreshResponseSchema, _)),
              Runtime.runPromise(runtime),
            ),
        });
      },
      { transport: { interceptors: [yield* effectInterceptor(mockInterceptor)] } },
    );
  }),
);

const MagicClientTest = Layer.effect(
  MagicClient,
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<Faker | EmailRef>();
    return {
      auth: {
        loginWithMagicLink: (request) =>
          Effect.gen(function* () {
            yield* Effect.flatMap(EmailRef, Ref.set(request.email));
            const faker = yield* Faker;
            return faker.string.uuid();
          }).pipe(Runtime.runPromise(runtime)) as PromiEvent<string>,
      } as Partial<Magic['auth']>,
      user: {
        logout: () => Promise.resolve(true),
      },
    } as Magic;
  }),
);

const fakeScalar = (faker: (typeof Faker)['Service'], scalar: ScalarType, field: DescField) => {
  if (field.name.endsWith('Id')) {
    const id = Ulid.generate({ time: faker.date.anytime() });
    return id.bytes;
  }

  // https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md#scalar-fields
  switch (scalar) {
    case ScalarType.STRING:
      return faker.word.words();

    case ScalarType.BOOL:
      return faker.datatype.boolean();

    case ScalarType.BYTES:
      return new Uint8Array();

    case ScalarType.DOUBLE:
    case ScalarType.FLOAT:
      return faker.number.float();

    case ScalarType.INT32:
    case ScalarType.UINT32:
    case ScalarType.SINT32:
    case ScalarType.FIXED32:
    case ScalarType.SFIXED32:
      return faker.number.int({ min: 0, max: 2 ** 32 / 2 - 1 });

    case ScalarType.INT64:
    case ScalarType.UINT64:
    case ScalarType.SINT64:
    case ScalarType.FIXED64:
    case ScalarType.SFIXED64:
      return faker.number.bigInt({ min: 0, max: 2n ** 64n / 2n - 1n });
  }
};

const fakeEnum = (faker: (typeof Faker)['Service'], enum_: DescEnum) =>
  faker.number.int({
    min: 0,
    max: enum_.values.length - 1,
  });

const fakeMessage = (faker: (typeof Faker)['Service'], message: DescMessage): Message => {
  if (message.typeName === 'google.protobuf.Timestamp') {
    return timestampFromDate(faker.date.anytime());
  }

  const value = Record.map(message.field, (field) => {
    switch (field.fieldKind) {
      case 'message':
        return fakeMessage(faker, field.message);

      case 'scalar':
        return fakeScalar(faker, field.scalar, field);

      case 'enum':
        return fakeEnum(faker, field.enum);

      case 'list':
        return faker.helpers.multiple(() => {
          switch (field.listKind) {
            case 'message':
              return fakeMessage(faker, field.message);

            case 'scalar':
              return fakeScalar(faker, field.scalar, field);

            case 'enum':
              return fakeEnum(faker, field.enum);
          }
        });

      default:
        throw new Error('Unimplemented field kind');
    }
  });

  return create(message, value);
};

const ApiTransportTest = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    const faker = yield* Faker;
    return createRouterTransport(
      (router) => {
        files.forEach((file) => {
          file.services.forEach((service) => {
            const methods = Record.map(service.method, (method) => () => fakeMessage(faker, method.output));
            router.service(service, methods as ServiceImpl<never>);
          });
        });
      },
      {
        transport: {
          // Interceptor flow order is reversed
          interceptors: [yield* effectInterceptor(flow(mockInterceptor, authorizationInterceptor))],
        },
      },
    );
  }),
);

export const ApiTest = pipe(
  ApiTransportTest,
  Layer.provideMerge(AuthTransportTest),
  Layer.provideMerge(MagicClientTest),
  Layer.provide(EmailTest),
  Layer.provide(FakerLive),
);

const mockInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    Effect.gen(function* () {
      const response = yield* next(request);
      yield* Effect.logDebug(`Mocking ${request.url}`, { request, response });
      yield* Effect.sleep('500 millis');
      return response;
    });

const tokens = Effect.gen(function* () {
  const email = yield* Effect.flatMap(EmailRef, Ref.get);

  const accessToken = yield* pipe(
    AccessTokenPayload.make({
      token_type: 'access_token',
      exp: pipe(yield* DateTime.now, DateTime.add({ minutes: 1 }), DateTime.toDate),
      email,
    }),
    Schema.encode(AccessTokenPayload),
    Effect.map((_) => new UnsecuredJWT(_).encode()),
  );

  const refreshToken = yield* pipe(
    RefreshTokenPayload.make({
      token_type: 'refresh_token',
      exp: pipe(yield* DateTime.now, DateTime.add({ days: 1 }), DateTime.toDate),
    }),
    Schema.encode(RefreshTokenPayload),
    Effect.map((_) => new UnsecuredJWT(_).encode()),
  );

  return { accessToken, refreshToken };
});
