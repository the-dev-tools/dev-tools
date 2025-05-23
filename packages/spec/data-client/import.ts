import { MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { Endpoint } from '@data-client/endpoint';

import { ImportRequestSchema, ImportResponse, ImportResponseSchema } from '../dist/buf/typescript/import/v1/import_pb';
import { CollectionListEndpoint } from '../dist/meta/collection/v1/collection.endpoints';
import { FlowListEndpoint } from '../dist/meta/flow/v1/flow.endpoints';
import { EndpointProps } from './resource';
import { createMethodKey, fetchMethod } from './utils';

export const import$ = ({ method, name }: EndpointProps<typeof ImportRequestSchema, typeof ImportResponseSchema>) => {
  const fetchFunction = (transport: Transport, input: MessageInitShape<typeof ImportRequestSchema>) =>
    fetchMethod(transport, method, input);

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  return new Endpoint(fetchFunction, {
    key,
    name,
    schema: {
      ...({} as ImportResponse),
      collection: CollectionListEndpoint.schema.items.push,
      flow: FlowListEndpoint.schema.items.push,
    },
    sideEffect: true,
  });
};
