import { createQueryOptions, useTransport } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, ToOptions, useMatchRoute } from '@tanstack/react-router';
import { Match, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { createContext, RefObject, useContext, useMemo, useRef, useState } from 'react';
import { MenuTrigger, Text, UNSTABLE_Tree as Tree } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal, FiRotateCw } from 'react-icons/fi';
import { MdLightbulbOutline } from 'react-icons/md';
import { twJoin } from 'tailwind-merge';

import { useConnectMutation, useConnectQuery } from '@the-dev-tools/api/connect-query';
import { Endpoint, EndpointListItem } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import {
  endpointCreate,
  endpointDelete,
  endpointDuplicate,
  endpointUpdate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleListItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleCreate,
  exampleDelete,
  exampleDuplicate,
  exampleList,
  exampleUpdate,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { Folder, FolderListItem } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import {
  folderCreate,
  folderDelete,
  folderUpdate,
} from '@the-dev-tools/spec/collection/item/folder/v1/folder-FolderService_connectquery';
import { CollectionItem, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import {
  collectionDelete,
  collectionList,
  collectionUpdate,
} from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { useEscapePortal } from '@the-dev-tools/ui/utils';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

const useInvalidateCollectionListQuery = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const collectionListQueryOptions = createQueryOptions(collectionList, { workspaceId }, { transport });
  return () => queryClient.invalidateQueries(collectionListQueryOptions);
};

interface CollectionListTreeContext {
  navigate?: boolean;
  showControls?: boolean;
  containerRef: RefObject<HTMLDivElement | null>;
}

const CollectionListTreeContext = createContext({} as CollectionListTreeContext);

class TreeKey extends Schema.Class<TreeKey>('CollectionListTreeKey')({
  collectionId: pipe(Schema.Uint8Array, Schema.optional),
  folderId: pipe(Schema.Uint8Array, Schema.optional),
  endpointId: pipe(Schema.Uint8Array, Schema.optional),
  exampleId: pipe(Schema.Uint8Array, Schema.optional),
}) {}

interface CollectionListTreeProps extends Omit<CollectionListTreeContext, 'containerRef'> {
  onAction?: (key: TreeKey) => void;
}

export const CollectionListTree = ({ onAction, ...context }: CollectionListTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const collectionListQuery = useConnectQuery(collectionList, { workspaceId });

  const ref = useRef<HTMLDivElement>(null);

  if (!collectionListQuery.isSuccess) return null;
  const collections = collectionListQuery.data.items;

  return (
    <CollectionListTreeContext.Provider value={{ ...context, containerRef: ref }}>
      <div ref={ref} className={tw`relative`}>
        <Tree
          aria-label='Collections'
          items={collections}
          onAction={
            onAction !== undefined
              ? (keyUnknown) => {
                  if (typeof keyUnknown !== 'string') return;
                  const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyUnknown));
                  onAction(key);
                }
              : undefined!
          }
        >
          {(_) => {
            const collectionIdCan = Ulid.construct(_.collectionId).toCanonical();
            return <CollectionTree id={collectionIdCan} collection={_} />;
          }}
        </Tree>
      </div>
    </CollectionListTreeContext.Provider>
  );
};

interface CollectionTreeProps {
  id: string;
  collection: CollectionListItem;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const { showControls, containerRef } = useContext(CollectionListTreeContext);

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId }, { enabled });
  const collectionDeleteMutation = useConnectMutation(collectionDelete);
  const collectionUpdateMutation = useConnectMutation(collectionUpdate);

  const folderCreateMutation = useConnectMutation(folderCreate);
  const endpointCreateMutation = useConnectMutation(endpointCreate);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: collection.name,
    onSuccess: (_) => collectionUpdateMutation.mutateAsync({ workspaceId, collectionId, name: _ }),
  });

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  return (
    <TreeItem
      id={pipe(new TreeKey({ collectionId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      textValue={collection.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      wrapperOnContextMenu={onContextMenu}
    >
      {collectionItemListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <Text ref={escape.ref} className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)}>
        {collection.name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={collectionUpdateMutation.isPending}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => void endpointCreateMutation.mutate({ collectionId, name: 'New API call' })}>
              Add Request
            </MenuItem>

            <MenuItem onAction={() => void folderCreateMutation.mutate({ collectionId, name: 'New folder' })}>
              Add Folder
            </MenuItem>

            <MenuItem
              variant='danger'
              onAction={() => void collectionDeleteMutation.mutate({ workspaceId, collectionId })}
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </TreeItem>
  );
};

const mapCollectionItemTree =
  (collectionId: Collection['collectionId'], parentFolderId?: Folder['folderId']) => (item: CollectionItem) =>
    pipe(
      Match.value(item),
      Match.when({ kind: ItemKind.FOLDER }, (_) => {
        const folderIdCan = Ulid.construct(_.folder!.folderId).toCanonical();
        return (
          <FolderTree id={folderIdCan} collectionId={collectionId} parentFolderId={parentFolderId} folder={_.folder!} />
        );
      }),
      Match.when({ kind: ItemKind.ENDPOINT }, (_) => {
        const endpointIdCan = Ulid.construct(_.endpoint!.endpointId).toCanonical();
        return (
          <EndpointTree
            id={endpointIdCan}
            collectionId={collectionId}
            parentFolderId={parentFolderId}
            endpoint={_.endpoint!}
            example={_.example!}
          />
        );
      }),
      Match.orElse(() => null),
    );

interface FolderTreeProps {
  id: string;
  collectionId: Collection['collectionId'];
  parentFolderId: Folder['folderId'] | undefined;
  folder: FolderListItem;
}

const FolderTree = ({ collectionId, parentFolderId, folder: { folderId, ...folder } }: FolderTreeProps) => {
  const { showControls, containerRef } = useContext(CollectionListTreeContext);

  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId, folderId }, { enabled });

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  const folderCreateMutation = useConnectMutation(folderCreate);
  const folderUpdateMutation = useConnectMutation(folderUpdate);
  const folderDeleteMutation = useConnectMutation(folderDelete);

  const endpointCreateMutation = useConnectMutation(endpointCreate);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: folder.name,
    onSuccess: (_) =>
      folderUpdateMutation.mutateAsync({
        collectionId,
        folderId,
        parentFolderId: parentFolderId!,
        name: _,
      }),
  });

  return (
    <TreeItem
      id={pipe(new TreeKey({ collectionId, folderId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      textValue={folder.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId, folderId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      wrapperOnContextMenu={onContextMenu}
    >
      {({ isExpanded }) => (
        <>
          {collectionItemListQuery.isLoading && (
            <Button variant='ghost' isDisabled className={tw`p-1`}>
              <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
            </Button>
          )}

          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text ref={escape.ref} className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)}>
            {folder.name}
          </Text>

          {isEditing &&
            escape.render(
              <TextField
                className={tw`w-full`}
                inputClassName={tw`-my-1 py-1`}
                isDisabled={folderUpdateMutation.isPending}
                {...textFieldProps}
              />,
            )}

          {showControls && (
            <MenuTrigger {...menuTriggerProps}>
              <Button variant='ghost' className={tw`p-0.5`}>
                <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
              </Button>

              <Menu {...menuProps}>
                <MenuItem onAction={() => void edit()}>Rename</MenuItem>

                <MenuItem
                  onAction={() =>
                    void endpointCreateMutation.mutate({
                      collectionId,
                      parentFolderId: folderId,
                      name: 'New API call',
                    })
                  }
                >
                  Add Request
                </MenuItem>

                <MenuItem
                  onAction={() =>
                    void folderCreateMutation.mutate({
                      collectionId,
                      parentFolderId: folderId,
                      name: 'New folder',
                    })
                  }
                >
                  Add Folder
                </MenuItem>

                <MenuItem
                  variant='danger'
                  onAction={() =>
                    void folderDeleteMutation.mutate({ collectionId, folderId, parentFolderId: parentFolderId! })
                  }
                >
                  Delete
                </MenuItem>
              </Menu>
            </MenuTrigger>
          )}
        </>
      )}
    </TreeItem>
  );
};

interface EndpointTreeProps {
  id: string;
  collectionId: Collection['collectionId'];
  parentFolderId: Folder['folderId'] | undefined;
  endpoint: EndpointListItem;
  example: ExampleListItem;
}

const EndpointTree = ({ id: endpointIdCan, collectionId, parentFolderId, endpoint, example }: EndpointTreeProps) => {
  const { endpointId, method, name } = endpoint;
  const { exampleId, lastResponseId } = example;

  const matchRoute = useMatchRoute();

  const { navigate = false, showControls, containerRef } = useContext(CollectionListTreeContext);

  const exampleIdCan = Ulid.construct(exampleId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const [enabled, setEnabled] = useState(false);

  const exampleListQuery = useConnectQuery(exampleList, { endpointId }, { enabled });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const exampleCreateMutation = useConnectMutation(exampleCreate);
  const endpointUpdateMutation = useConnectMutation(endpointUpdate);
  const endpointDeleteMutation = useConnectMutation(endpointDelete);
  const endpointDuplicateMutation = useConnectMutation(endpointDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: endpoint.name,
    onSuccess: (_) => endpointUpdateMutation.mutateAsync({ collectionId, endpointId, name: _ }),
  });

  const route = {
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
  } satisfies ToOptions;

  return (
    <TreeItem
      id={pipe(new TreeKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      textValue={name}
      href={navigate ? route : undefined!}
      isActive={navigate && matchRoute(route) !== false}
      childItems={exampleListQuery.data?.items ?? []}
      childItem={(_) => {
        const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
        return <ExampleItem id={exampleIdCan} collectionId={collectionId} endpointId={endpointId} example={_} />;
      }}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      wrapperOnContextMenu={onContextMenu}
    >
      {exampleListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <MethodBadge method={method} />

      <Text ref={escape.ref} className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)}>
        {name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={endpointUpdateMutation.isPending}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={() =>
                void exampleCreateMutation.mutate({
                  endpointId,
                  name: 'New Example',
                })
              }
            >
              Add Example
            </MenuItem>

            <MenuItem onAction={() => void endpointDuplicateMutation.mutate({ endpointId })}>Duplicate</MenuItem>

            <MenuItem
              variant='danger'
              onAction={() =>
                void endpointDeleteMutation.mutate({
                  collectionId,
                  endpointId,
                  parentFolderId: parentFolderId!,
                })
              }
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </TreeItem>
  );
};

interface ExampleItemProps {
  id: string;
  collectionId: Collection['collectionId'];
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
}

const ExampleItem = ({ id: exampleIdCan, collectionId, endpointId, example }: ExampleItemProps) => {
  const { exampleId, lastResponseId, name } = example;

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const matchRoute = useMatchRoute();

  const { navigate = false, showControls, containerRef } = useContext(CollectionListTreeContext);

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleUpdateMutation = useConnectMutation(exampleUpdate);
  const exampleDeleteMutation = useConnectMutation(exampleDelete);
  const exampleDuplicateMutation = useConnectMutation(exampleDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: name,
    onSuccess: (_) => exampleUpdateMutation.mutateAsync({ endpointId, exampleId, name: _ }),
  });

  const route = {
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
  } satisfies ToOptions;

  return (
    <TreeItem
      id={pipe(new TreeKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      textValue={name}
      href={navigate ? route : undefined!}
      isActive={navigate && matchRoute(route) !== false}
      wrapperOnContextMenu={onContextMenu}
    >
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text ref={escape.ref} className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)}>
        {name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={exampleUpdateMutation.isPending}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => void exampleDuplicateMutation.mutate({ exampleId })}>Duplicate</MenuItem>

            <MenuItem variant='danger' onAction={() => void exampleDeleteMutation.mutate({ endpointId, exampleId })}>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </TreeItem>
  );
};
