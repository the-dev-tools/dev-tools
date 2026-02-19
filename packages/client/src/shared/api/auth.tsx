import { MessageInitShape } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { KeyValueStore } from '@effect/platform';
import { Config, DateTime, Effect, Option, pipe, Schema } from 'effect';
import {
  AuthService as AuthConnectService,
  SignInRequestSchema,
  SignUpRequestSchema,
} from '@the-dev-tools/spec/buf/api/auth/v1/auth_pb';
import { InterceptorNext, InterceptorRequest, requestEffect } from './connect-rpc';
import { defaultInterceptors } from './interceptors';
import { registry } from './protobuf';

class AuthTransport extends Effect.Service<AuthTransport>()('AuthTransport', {
  effect: Effect.gen(function* () {
    const baseUrl = yield* pipe(Config.string('PUBLIC_AUTH_URL'), Config.withDefault('http://localhost:8081'));

    return createConnectTransport({
      baseUrl,
      interceptors: defaultInterceptors,
      jsonOptions: { registry },
      useHttpGet: true,
    });
  }),
}) {}

class AuthStoreData extends Schema.Class<AuthStoreData>('AuthStoreData')({
  accessToken: Schema.String,
  refreshToken: Schema.String,
  userId: Schema.Uint8Array,
}) {}

const setAuthData = Effect.fn(function* (data?: AuthStoreData) {
  const kv = yield* KeyValueStore.KeyValueStore;

  if (data) {
    const store = yield* Schema.encode(Schema.parseJson(AuthStoreData))(data);
    yield* kv.set('AUTH', store);
  } else {
    yield* kv.remove('AUTH');
  }

  location.reload();
});

export interface AuthData {
  accessToken: string;
  name: string;
  refreshToken: string;
  userId: Uint8Array;
}

const getAuthData = Effect.gen(function* () {
  const kv = yield* KeyValueStore.KeyValueStore;
  const store = yield* kv.get('AUTH');

  if (Option.isNone(store)) return Option.none<AuthData>();

  let { accessToken, refreshToken, userId } = yield* Schema.decode(Schema.parseJson(AuthStoreData))(store.value);

  const { expiresAt, name } = yield* decodePayload(accessToken.split('.')[1]);

  if (yield* DateTime.isFuture(expiresAt)) return Option.some<AuthData>({ accessToken, name, refreshToken, userId });

  const transport = yield* AuthTransport;

  ({
    message: { accessToken, refreshToken },
  } = yield* requestEffect({
    input: { refreshToken },
    method: AuthConnectService.method.refreshToken,
    transport,
  }));

  yield* setAuthData({ accessToken, refreshToken, userId });

  return Option.some<AuthData>({ accessToken, name, refreshToken, userId });
});

const decodePayload = pipe(
  Schema.Struct({
    expiresAt: Schema.DateTimeUtc,
    name: Schema.String,
  }),
  (_) => Schema.parseJson(_),
  (_) => Schema.compose(Schema.StringFromBase64, _),
  Schema.decodeUnknown,
);

export const authInterceptor = Effect.fn(function* (next: InterceptorNext, request: InterceptorRequest) {
  const { getAuthData } = yield* AuthService;
  const authData = yield* getAuthData;
  if (Option.isSome(authData)) request.header.set('Authorization', `Bearer ${authData.value.accessToken}`);
  return yield* Effect.tryPromise(() => next(request));
});

export class AuthService extends Effect.Service<AuthService>()('AuthService', {
  accessors: true,
  dependencies: [AuthTransport.Default],
  effect: Effect.gen(function* () {
    const transport = yield* AuthTransport;

    const getAuthDataCached = yield* Effect.cachedWithTTL(getAuthData, '100 millis');

    const signUp = Effect.fn(function* (input: MessageInitShape<typeof SignUpRequestSchema>) {
      const { message } = yield* requestEffect({ input, method: AuthConnectService.method.signUp, transport });
      yield* setAuthData(message);
    });

    const signIn = Effect.fn(function* (input: MessageInitShape<typeof SignInRequestSchema>) {
      const { message } = yield* requestEffect({ input, method: AuthConnectService.method.signIn, transport });
      yield* setAuthData(message);
    });

    const signOut = Effect.gen(function* () {
      const { refreshToken } = yield* Effect.flatten(getAuthDataCached);
      yield* requestEffect({ input: { refreshToken }, method: AuthConnectService.method.signOut, transport });
      yield* setAuthData();
    });

    return { getAuthData: getAuthDataCached, signIn, signOut, signUp };
  }),
}) {}
