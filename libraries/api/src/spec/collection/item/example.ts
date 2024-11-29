import {
  exampleCreate,
  exampleDelete,
  exampleGet,
  exampleList,
  exampleUpdate,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';

import { MutationSpec } from '../../../query.internal';

export const exampleCreateSpec = {
  mutation: exampleCreate,
  key: 'exampleId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId'],
  onSuccess: [
    ['query - get - add cache', { query: exampleGet }],
    ['query - list - add item cache', { query: exampleList }],
  ],
} satisfies MutationSpec;

export const exampleUpdateSpec = {
  mutation: exampleUpdate,
  key: 'exampleId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId'],
  onSuccess: [
    ['query - get - update cache', { query: exampleGet }],
    ['query - list - update item cache', { query: exampleList }],
  ],
} satisfies MutationSpec;

export const exampleDeleteSpec = {
  mutation: exampleDelete,
  key: 'exampleId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId'],
  onSuccess: [
    ['query - get - delete cache', { query: exampleGet }],
    ['query - list - delete item cache', { query: exampleList }],
  ],
} satisfies MutationSpec;
