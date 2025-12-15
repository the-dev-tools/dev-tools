import { Config, Effect, pipe, Schedule } from 'effect';
import { HealthService } from '@the-dev-tools/spec/buf/api/health/v1/health_pb';
import { Connect, Protobuf } from '~/api';
import { defaultInterceptors } from './interceptors';
import { ApiTransportMock } from './mock';

export class ApiTransport extends Effect.Service<ApiTransport>()('ApiTransport', {
  dependencies: [ApiTransportMock.Default],
  effect: Effect.gen(function* () {
    const mock = yield* pipe(Config.boolean('PUBLIC_MOCK'), Config.withDefault(false));
    if (mock) return yield* ApiTransportMock;

    const transport = Connect.createConnectTransport({
      baseUrl: 'server://',
      interceptors: defaultInterceptors,
      jsonOptions: { registry: Protobuf.registry },
      useHttpGet: true,
    });

    // Wait for the server to start up
    yield* pipe(
      Effect.tryPromise((signal) =>
        Connect.request({
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
