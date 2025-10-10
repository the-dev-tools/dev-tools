import { createRegistry } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { Config, Effect, pipe } from 'effect';
import { files } from '@the-dev-tools/spec/files';
import { defaultInterceptors } from './interceptors';
import { ApiTransportMock } from './mock';

export class ApiTransport extends Effect.Service<ApiTransport>()('ApiTransport', {
  dependencies: [ApiTransportMock.Default],
  effect: Effect.gen(function* () {
    yield* Effect.log('transport created');
    const mock = yield* pipe(Config.boolean('PUBLIC_MOCK'), Config.withDefault(false));
    if (mock) return yield* ApiTransportMock;

    return createConnectTransport({
      baseUrl: 'http://localhost:8080',
      interceptors: defaultInterceptors,
      jsonOptions: { registry: createRegistry(...files) },
      useHttpGet: true,
    });
  }),
}) {}
