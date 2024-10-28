import {
  workspaceCreate,
  workspaceDelete,
  workspaceGet,
  workspaceList,
  workspaceMemberCreate,
  workspaceMemberDelete,
  workspaceMemberList,
  workspaceMemberUpdate,
  workspaceUpdate,
} from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';

import { MutationSpec } from '../query.internal';

export const workspaceCreateSpec = {
  mutation: workspaceCreate,
  key: 'workspaceId',
  onSuccess: [
    ['query - get - add cache', { query: workspaceGet }],
    ['query - list - add item cache', { query: workspaceList }],
  ],
} satisfies MutationSpec;

export const workspaceUpdateSpec = {
  mutation: workspaceUpdate,
  key: 'workspaceId',
  onSuccess: [
    ['query - get - update cache', { query: workspaceGet }],
    ['query - list - update item cache', { query: workspaceList }],
  ],
} satisfies MutationSpec;

export const workspaceDeleteSpec = {
  mutation: workspaceDelete,
  key: 'workspaceId',
  onSuccess: [
    ['query - get - delete cache', { query: workspaceGet }],
    ['query - list - delete item cache', { query: workspaceList }],
  ],
} satisfies MutationSpec;

export const workspaceMemberCreateSpec = {
  mutation: workspaceMemberCreate,
  key: 'workspaceMemberId',
  parentKeys: ['workspaceId'],
  onSuccess: [['query - list - add item cache', { query: workspaceMemberList }]],
} satisfies MutationSpec;

export const workspaceMemberUpdateSpec = {
  mutation: workspaceMemberUpdate,
  key: 'workspaceMemberId',
  parentKeys: ['workspaceId'],
  onSuccess: [['query - list - update item cache', { query: workspaceMemberList }]],
} satisfies MutationSpec;

export const workspaceMemberDeleteSpec = {
  mutation: workspaceMemberDelete,
  key: 'workspaceMemberId',
  parentKeys: ['workspaceId'],
  onSuccess: [['query - list - delete item cache', { query: workspaceMemberList }]],
} satisfies MutationSpec;
