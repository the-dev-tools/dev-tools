import { Transport } from '@connectrpc/connect';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { Schema } from '@effect/schema';
import { Context, Data, DateTime, Effect, Option, pipe } from 'effect';
import { decodeJwt } from 'jose';
import { LoginWithMagicLinkConfiguration, Magic } from 'magic-sdk';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { OrganizationService } from '@the-dev-tools/protobuf/organization/v1/organization_connect';

import { accessTokenKey, AccessTokenPayload, JWTPayload, refreshTokenKey, RefreshTokenPayload } from './jwt';
import { AnyFnEffect, ApiTransport, Request } from './transport';

export class AuthTransport extends Context.Tag('AuthTransport')<AuthTransport, Transport>() {}

export class MagicClient extends Context.Tag('MagicClient')<MagicClient, Magic>() {}

export class NoOrganizationSelectedError extends Data.TaggedError('NoOrganizationSelectedError') {}

export const login = (configuration: LoginWithMagicLinkConfiguration) =>
  Effect.gen(function* () {
    // Authenticate using Magic SDK
    const magicClient = yield* MagicClient;
    const didToken = yield* pipe(
      Effect.tryPromise(() => magicClient.auth.loginWithMagicLink(configuration)),
      Effect.flatMap(Effect.fromNullable),
    );

    // Authorize
    const authTransport = yield* AuthTransport;
    const loginResponse = yield* Effect.tryPromise((signal) =>
      authTransport.unary(AuthService, AuthService.methods.dID, signal, undefined, undefined, { didToken }),
    );
    const { accessToken, refreshToken } = loginResponse.message;

    // Validate tokens
    yield* pipe(
      Effect.try(() => decodeJwt<typeof AccessTokenPayload.Encoded>(accessToken)),
      Effect.flatMap(Schema.decode(AccessTokenPayload)),
    );
    yield* pipe(
      Effect.try(() => decodeJwt<typeof RefreshTokenPayload.Encoded>(refreshToken)),
      Effect.flatMap(Schema.decode(RefreshTokenPayload)),
    );

    // Store tokens
    const store = yield* KeyValueStore;
    yield* store.forSchema(Schema.String).set(accessTokenKey, accessToken);
    yield* store.forSchema(Schema.String).set(refreshTokenKey, refreshToken);

    // Select first organization if user has exactly one
    const transport = yield* ApiTransport;
    const organizationsResponse = yield* Effect.tryPromise((signal) =>
      transport.unary(
        OrganizationService,
        OrganizationService.methods.getOrganizations,
        signal,
        undefined,
        undefined,
        {},
      ),
    );
    const { organizations } = organizationsResponse.message;
    if (organizations.length !== 1) return yield* new NoOrganizationSelectedError();
    yield* setOrganizationId(organizations[0]!.organizationId);
  });

export const logout = Effect.gen(function* () {
  const magicClient = yield* MagicClient;
  yield* Effect.tryPromise(() => magicClient.user.logout());
  const store = yield* KeyValueStore;
  yield* store.remove(accessTokenKey);
  yield* store.remove(refreshTokenKey);
  yield* store.remove(organizationIdKey);
});

const organizationIdKey = 'OrganizationId';

export const getOrganizationId = Effect.gen(function* () {
  const store = yield* KeyValueStore;
  return yield* pipe(
    store.forSchema(Schema.String).get(organizationIdKey),
    Effect.flatMap(Effect.orElseFail(() => new NoOrganizationSelectedError())),
  );
});

export const setOrganizationId = (id: string) =>
  Effect.gen(function* () {
    const store = yield* KeyValueStore;
    yield* store.forSchema(Schema.String).set(organizationIdKey, id);
  });

const isTokenExpired = (token: string) =>
  pipe(
    Effect.try(() => decodeJwt<typeof JWTPayload.Encoded>(token)),
    Effect.flatMap(Schema.decode(JWTPayload)),
    Effect.flatMap((_) => DateTime.make(_.exp)),
    Effect.flatMap(DateTime.isPast),
  );

const accessToken = Effect.gen(function* () {
  const store = yield* KeyValueStore;
  let accessToken = yield* pipe(store.forSchema(Schema.String).get(accessTokenKey), Effect.flatten);
  const accessTokenExpired = yield* isTokenExpired(accessToken);

  if (!accessTokenExpired) return accessToken;

  let refreshToken = yield* pipe(store.forSchema(Schema.String).get(refreshTokenKey), Effect.flatten);
  const refreshTokenExpired = yield* isTokenExpired(refreshToken);

  if (refreshTokenExpired) {
    yield* logout;
    return yield* Effect.fail('Authorization expired' as const);
  }

  const transport = yield* AuthTransport;
  const response = yield* Effect.tryPromise((signal) =>
    transport.unary(AuthService, AuthService.methods.refreshToken, signal, undefined, undefined, {
      refreshToken,
    }),
  );
  ({ accessToken, refreshToken } = response.message);

  yield* store.forSchema(Schema.String).set(accessTokenKey, accessToken);
  yield* store.forSchema(Schema.String).set(refreshTokenKey, refreshToken);

  return accessToken;
});

export const authorizationInterceptor =
  <E, R>(next: AnyFnEffect<E, R>) =>
  (request: Request) =>
    Effect.gen(function* () {
      const store = yield* KeyValueStore;

      request.header.set('Authorization', `Bearer ${yield* accessToken}`);

      const organizationId = yield* store.forSchema(Schema.String).get(organizationIdKey);
      if (Option.isSome(organizationId)) {
        request.header.set('organization_id', organizationId.value);
      }

      return yield* next(request);
    });

export const getUser = pipe(
  accessToken,
  Effect.flatMap((_) => Effect.try(() => decodeJwt<typeof AccessTokenPayload.Encoded>(_))),
  Effect.flatMap(Schema.decode(AccessTokenPayload)),
);
