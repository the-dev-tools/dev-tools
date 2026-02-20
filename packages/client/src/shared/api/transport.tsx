import { createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, pipe, Schedule } from 'effect';
import { HealthService } from '@the-dev-tools/spec/buf/api/health/v1/health_pb';
import { authInterceptor, AuthToken } from './auth.internal';
import { effectInterceptor, request } from './connect-rpc';
import { defaultInterceptors } from './interceptors';
import { ApiTransportMock } from './mock';
import { registry } from './protobuf';

export class ApiTransport extends Effect.Service<ApiTransport>()('ApiTransport', {
  dependencies: [ApiTransportMock.Default, AuthToken.Default],
  effect: Effect.gen(function* () {
    const mock = yield* pipe(Config.boolean('PUBLIC_MOCK'), Config.withDefault(false));
    if (mock) return yield* ApiTransportMock;

    const transport = createConnectTransport({
      baseUrl: 'server://',
      interceptors: [yield* effectInterceptor(authInterceptor), ...defaultInterceptors],
      jsonOptions: { registry },
      useHttpGet: true,
    });

    // Wait for the server to start up
    yield* pipe(
      Effect.tryPromise((signal) =>
        request({
          method: HealthService.method.healthCheck,
          signal,
          timeoutMs: 0,
          transport,
        }),
      ),
      Effect.retry({
        schedule: Schedule.exponential('10 millis'),
        times: 100,
      }),
    );

    return transport;
  }),
}) {}
