import { createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, pipe, Schedule } from 'effect';
import { HealthService } from '@the-dev-tools/spec/buf/api/health/v1/health_pb';
import { request } from './connect-rpc';
import { defaultInterceptors } from './interceptors';
import { registry } from './protobuf';

// The mock transport (`./mock`) transitively imports `@faker-js/faker` (~3 MB).
// Load it lazily via dynamic import so the faker payload is code-split into its
// own chunk and the main bundle stays lean when PUBLIC_MOCK is off (prod path).

export class ApiTransport extends Effect.Service<ApiTransport>()('ApiTransport', {
  effect: Effect.gen(function* () {
    const mock = yield* pipe(Config.boolean('PUBLIC_MOCK'), Config.withDefault(false));
    if (mock) {
      const { ApiTransportMock } = yield* Effect.promise(() => import('./mock'));
      return yield* Effect.provide(ApiTransportMock, ApiTransportMock.Default);
    }

    const transport = createConnectTransport({
      baseUrl: 'server://',
      interceptors: defaultInterceptors,
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
        schedule: Schedule.union(Schedule.exponential('100 millis'), Schedule.spaced('2 seconds')),
        times: 60,
      }),
    );

    return transport;
  }),
}) {}
