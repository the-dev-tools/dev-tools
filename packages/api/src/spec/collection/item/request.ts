import {
  assertCreate,
  assertDelete,
  assertList,
  assertUpdate,
  queryCreate,
  queryDelete,
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';

import { MutationSpec } from '../../../query.internal';

export const assertCreateSpec = {
  mutation: assertCreate,
  key: 'assertId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - add item cache', { query: assertList }]],
} satisfies MutationSpec;

export const assertUpdateSpec = {
  mutation: assertUpdate,
  key: 'assertId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - update item cache', { query: assertList }]],
} satisfies MutationSpec;

export const assertDeleteSpec = {
  mutation: assertDelete,
  key: 'assertId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - delete item cache', { query: assertList }]],
} satisfies MutationSpec;

export const queryCreateSpec = {
  mutation: queryCreate,
  key: 'queryId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - add item cache', { query: queryList }]],
} satisfies MutationSpec;

export const queryUpdateSpec = {
  mutation: queryUpdate,
  key: 'queryId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - update item cache', { query: queryList }]],
} satisfies MutationSpec;

export const queryDeleteSpec = {
  mutation: queryDelete,
  key: 'queryId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId', 'exampleId'],
  onSuccess: [['query - list - delete item cache', { query: queryList }]],
} satisfies MutationSpec;
