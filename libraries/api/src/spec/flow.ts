import {
  flowCreate,
  flowDelete,
  flowGet,
  flowList,
  flowUpdate,
} from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';

import { MutationSpec } from '../query.internal';

export const flowCreateSpec = {
  mutation: flowCreate,
  key: 'flowId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - add cache', { query: flowGet }],
    ['query - list - add item cache', { query: flowList }],
  ],
} satisfies MutationSpec;

export const flowUpdateSpec = {
  mutation: flowUpdate,
  key: 'flowId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - update cache', { query: flowGet }],
    ['query - list - update item cache', { query: flowList }],
  ],
} satisfies MutationSpec;

export const flowDeleteSpec = {
  mutation: flowDelete,
  key: 'flowId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - delete cache', { query: flowGet }],
    ['query - list - delete item cache', { query: flowList }],
  ],
} satisfies MutationSpec;
