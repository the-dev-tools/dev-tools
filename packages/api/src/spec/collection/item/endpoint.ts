import {
  endpointCreate,
  endpointDelete,
  endpointUpdate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';

import { MutationSpec } from '../../../query.internal';

export const endpointCreateSpec = {
  mutation: endpointCreate,
  key: 'endpointId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - add item cache', { query: collectionItemList }]],
} satisfies MutationSpec;

export const endpointUpdateSpec = {
  mutation: endpointUpdate,
  key: 'endpointId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - update item cache', { query: collectionItemList }]],
} satisfies MutationSpec;

export const endpointDeleteSpec = {
  mutation: endpointDelete,
  key: 'endpointId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - delete item cache', { query: collectionItemList }]],
} satisfies MutationSpec;
