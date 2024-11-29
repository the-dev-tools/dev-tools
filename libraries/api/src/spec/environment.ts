import {
  environmentCreate,
  environmentDelete,
  environmentGet,
  environmentList,
  environmentUpdate,
} from '@the-dev-tools/spec/environment/v1/environment-EnvironmentService_connectquery';

import { MutationSpec } from '../query.internal';

export const environmentCreateSpec = {
  mutation: environmentCreate,
  key: 'environmentId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - add cache', { query: environmentGet }],
    ['query - list - add item cache', { query: environmentList }],
  ],
} satisfies MutationSpec;

export const environmentUpdateSpec = {
  mutation: environmentUpdate,
  key: 'environmentId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - update cache', { query: environmentGet }],
    ['query - list - update item cache', { query: environmentList }],
  ],
} satisfies MutationSpec;

export const environmentDeleteSpec = {
  mutation: environmentDelete,
  key: 'environmentId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - delete cache', { query: environmentGet }],
    ['query - list - delete item cache', { query: environmentList }],
  ],
} satisfies MutationSpec;
