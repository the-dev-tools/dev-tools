import { Endpoint } from '@data-client/endpoint';

import { ImportResponse, ImportService } from '../dist/buf/typescript/import/v1/import_pb';
import { CollectionListEndpoint } from '../dist/meta/collection/v1/collection.endpoints';
import { FlowListEndpoint } from '../dist/meta/flow/v1/flow.endpoints';
import { MakeEndpointProps } from './resource';
import { makeEndpointFn, makeKey } from './utils';

export const import$ = ({ method, name }: MakeEndpointProps<typeof ImportService.method.import>) =>
  new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    schema: {
      ...({} as ImportResponse),
      collection: CollectionListEndpoint.schema.items.push,
      flow: FlowListEndpoint.schema.items.push,
    },
    sideEffect: true,
  });
