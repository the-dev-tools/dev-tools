import {
  folderCreate,
  folderDelete,
  folderUpdate,
} from '@the-dev-tools/spec/collection/item/folder/v1/folder-FolderService_connectquery';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';

import { MutationSpec } from '../../../query.internal';

export const folderCreateSpec = {
  mutation: folderCreate,
  key: 'folderId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - add item cache', { query: collectionItemList }]],
} satisfies MutationSpec;

export const folderUpdateSpec = {
  mutation: folderUpdate,
  key: 'folderId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - update item cache', { query: collectionItemList }]],
} satisfies MutationSpec;

export const folderDeleteSpec = {
  mutation: folderDelete,
  key: 'folderId',
  parentKeys: ['workspaceId', 'collectionId'],
  onSuccess: [['query - list - delete item cache', { query: collectionItemList }]],
} satisfies MutationSpec;
