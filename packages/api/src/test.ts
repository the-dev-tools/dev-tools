import { createRouterTransport } from '@connectrpc/connect';
import { Schema } from '@effect/schema';
import { Context, DateTime, Effect, flow, Layer, Match, pipe, Ref, Runtime, String } from 'effect';
import { UnsecuredJWT } from 'jose';
import { Magic, PromiEvent } from 'magic-sdk';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { AuthServiceDIDResponse } from '@the-dev-tools/protobuf/auth/v1/auth_pb';
import { CollectionService } from '@the-dev-tools/protobuf/collection/v1/collection_connect';
import {
  ApiCall,
  Folder,
  GetCollectionRequest,
  GetCollectionResponse,
  Item,
  ListCollectionsResponse,
  RunApiCallRequest,
  RunApiCallResponse,
} from '@the-dev-tools/protobuf/collection/v1/collection_pb';
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
          dID: () => Runtime.runPromise(runtime)(tokens),
          refreshToken: () => Runtime.runPromise(runtime)(tokens),
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

const ApiTransportTest = Layer.effect(
  ApiTransport,
  Effect.gen(function* () {
    const runtime = yield* Effect.runtime<Faker>();
    return createRouterTransport(
      (router) => {
        router.service(CollectionService, {
          listCollections: flow(listCollections, Runtime.runPromise(runtime)),
          getCollection: flow(getCollection, Runtime.runPromise(runtime)),
          runApiCall: flow(runApiCall, Runtime.runPromise(runtime)),
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
      exp: pipe(yield* DateTime.now, DateTime.add({ hours: 1 }), DateTime.toDate),
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

  return new AuthServiceDIDResponse({ accessToken, refreshToken });
});

const listCollections = () =>
  Effect.gen(function* () {
    const faker = yield* Faker;
    const metaCollections = yield* pipe(
      faker.helpers.multiple(() => meta(), { count: 10 }),
      Effect.all,
    );
    return new ListCollectionsResponse({ metaCollections });
  });

const getCollection = (request: GetCollectionRequest) =>
  Effect.gen(function* () {
    const meta_ = yield* meta(request.id);
    return new GetCollectionResponse({
      ...meta_,
      items: yield* items(meta_.id, undefined, 3),
    });
  });

const runApiCall = (request: RunApiCallRequest) =>
  Effect.gen(function* () {
    const faker = yield* Faker;
    return new RunApiCallResponse({
      result: {
        id: request.id,
        name: faker.internet.url(),
        duration: faker.number.bigInt(),
        response: {
          case: 'httpResponse',
          value: { statusCode: faker.internet.httpStatusCode() },
        },
      },
    });
  });

const meta = (id?: string) =>
  Effect.gen(function* () {
    const faker = yield* Faker;
    return {
      // TODO: Replace with ULID once implemented upstream to better match the backend
      // https://github.com/faker-js/faker/pull/2524
      id: id ?? faker.string.uuid(),
      name: pipe(faker.word.words({ count: { min: 1, max: 3 } }), String.capitalize),
    };
  });

const item = (collectionId: string, parentId: string | undefined, depth: number): Effect.Effect<Item, never, Faker> =>
  Effect.gen(function* () {
    const faker = yield* Faker;
    const case_ = depth > 0 ? faker.helpers.arrayElement(['apiCall', 'folder'] as const) : 'apiCall';
    const value = yield* pipe(
      Match.value(case_),
      Match.when('apiCall', () => apiCall),
      Match.when('folder', () => folder(collectionId, parentId, depth - 1)),
      Match.exhaustive,
    );
    return new Item({ data: { case: case_, value } });
  });

const items = (collectionId: string, parentId: string | undefined, depth: number) =>
  Effect.gen(function* () {
    const faker = yield* Faker;
    return yield* pipe(
      faker.helpers.multiple(() => item(collectionId, parentId, depth), { count: { min: 3, max: 10 } }),
      Effect.all,
    );
  });

const apiCall = Effect.gen(function* () {
  const faker = yield* Faker;
  return new ApiCall({
    meta: yield* meta(),
    collectionId: '',
    parentId: '',
    data: {
      url: faker.internet.url(),
      method: faker.internet.httpMethod(),
    },
  });
});

const folder = (collectionId: string, parentId: string | undefined, depth: number) =>
  Effect.gen(function* () {
    const meta_ = yield* meta();
    return new Folder({
      meta: meta_,
      collectionId,
      ...(parentId ? { parentId } : {}),
      items: yield* items(collectionId, meta_.id, depth),
    });
  });
