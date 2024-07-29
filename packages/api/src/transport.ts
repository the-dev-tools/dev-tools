import { Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { Context, Layer } from 'effect';

export class ApiTransport extends Context.Tag('ApiTransport')<ApiTransport, Transport>() {}

export const ApiTransportDev = Layer.succeed(
  ApiTransport,
  ApiTransport.of(
    createConnectTransport({
      baseUrl: 'https://devtools-backend.fly.dev',
      useHttpGet: true,
    }),
  ),
);
