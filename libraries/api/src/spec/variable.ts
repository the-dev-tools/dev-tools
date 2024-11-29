import {
  variableCreate,
  variableDelete,
  variableGet,
  variableList,
  variableUpdate,
} from '@the-dev-tools/spec/variable/v1/variable-VariableService_connectquery';

import { MutationSpec } from '../query.internal';

export const variableCreateSpec = {
  mutation: variableCreate,
  key: 'variableId',
  parentKeys: ['workspaceId', 'environmentId'],
  onSuccess: [
    ['query - get - add cache', { query: variableGet }],
    ['query - list - add item cache', { query: variableList }],
  ],
} satisfies MutationSpec;

export const variableUpdateSpec = {
  mutation: variableUpdate,
  key: 'variableId',
  parentKeys: ['workspaceId', 'environmentId'],
  onSuccess: [
    ['query - get - update cache', { query: variableGet }],
    ['query - list - update item cache', { query: variableList }],
  ],
} satisfies MutationSpec;

export const variableDeleteSpec = {
  mutation: variableDelete,
  key: 'variableId',
  parentKeys: ['workspaceId', 'environmentId'],
  onSuccess: [
    ['query - get - delete cache', { query: variableGet }],
    ['query - list - delete item cache', { query: variableList }],
  ],
} satisfies MutationSpec;
