import {
  collectionCreate,
  collectionDelete,
  collectionGet,
  collectionImportHar,
  collectionImportPostman,
  collectionList,
  collectionUpdate,
} from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';

import { MutationSpec } from '../query.internal';

export const collectionCreateSpec = {
  mutation: collectionCreate,
  key: 'collectionId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - add cache', { query: collectionGet }],
    ['query - list - add item cache', { query: collectionList }],
  ],
} satisfies MutationSpec;

export const collectionImportPostmanSpec = {
  mutation: collectionImportPostman,
  key: 'collectionId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - add cache', { query: collectionGet }],
    ['query - list - add item cache', { query: collectionList }],
  ],
} satisfies MutationSpec;

export const collectionImportHarSpec = {
  mutation: collectionImportHar,
  key: 'collectionId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - add cache', { query: collectionGet }],
    ['query - list - add item cache', { query: collectionList }],
  ],
} satisfies MutationSpec;

export const collectionUpdateSpec = {
  mutation: collectionUpdate,
  key: 'collectionId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - update cache', { query: collectionGet }],
    ['query - list - update item cache', { query: collectionList }],
  ],
} satisfies MutationSpec;

export const collectionDeleteSpec = {
  mutation: collectionDelete,
  key: 'collectionId',
  parentKeys: ['workspaceId'],
  onSuccess: [
    ['query - get - delete cache', { query: collectionGet }],
    ['query - list - delete item cache', { query: collectionList }],
  ],
} satisfies MutationSpec;
