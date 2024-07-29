import { Context, Layer } from 'effect';

import { AuthService } from '@the-dev-tools/protobuf/auth/v1/auth_connect';
import { CollectionService } from '@the-dev-tools/protobuf/collection/v1/collection_connect';
import { FlowService } from '@the-dev-tools/protobuf/flow/v1/flow_connect';

import { createEffectClient, EffectClient } from './effect-client';

export class ApiClient extends Context.Tag('ApiClient')<
  ApiClient,
  {
    auth: EffectClient<typeof AuthService>;
    collection: EffectClient<typeof CollectionService>;
    flow: EffectClient<typeof FlowService>;
  }
>() {}

export const ApiClientLive = Layer.succeed(
  ApiClient,
  ApiClient.of({
    auth: createEffectClient(AuthService),
    collection: createEffectClient(CollectionService),
    flow: createEffectClient(FlowService),
  }),
);
