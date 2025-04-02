import { createQueryOptions, useTransport } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, ToOptions, useMatchRoute } from '@tanstack/react-router';
import { Array, Match, Option, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { createContext, RefObject, useContext, useMemo, useRef, useState } from 'react';
import { MenuTrigger, Text, Tree } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal } from 'react-icons/fi';
import { MdLightbulbOutline } from 'react-icons/md';
import { twJoin } from 'tailwind-merge';

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
import { CollectionItem, ItemKind, ItemKindSchema } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import {
  collectionDelete,
  collectionList,
  collectionUpdate,
} from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { export$ } from '@the-dev-tools/spec/export/v1/export-ExportService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useConnectMutation, useConnectQuery, useConnectSuspenseQuery } from '~/api/connect-query';
import { enumToString } from '~/api/utils';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

const useInvalidateCollectionListQuery = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const collectionListQueryOptions = createQueryOptions(collectionList, { workspaceId }, { transport });
  return () => queryClient.invalidateQueries(collectionListQueryOptions);
};

interface CollectionListTreeContext {
  containerRef: RefObject<HTMLDivElement | null>;
  navigate?: boolean;
  showControls?: boolean;
}

const CollectionListTreeContext = createContext({} as CollectionListTreeContext);

class TreeKey extends Schema.Class<TreeKey>('CollectionListTreeKey')({
  collectionId: pipe(Schema.Uint8Array, Schema.optional),
  endpointId: pipe(Schema.Uint8Array, Schema.optional),
  exampleId: pipe(Schema.Uint8Array, Schema.optional),
  folderId: pipe(Schema.Uint8Array, Schema.optional),
}) {}

interface CollectionListTreeProps extends Omit<CollectionListTreeContext, 'containerRef'> {
  onAction?: (key: TreeKey) => void;
}

export const CollectionListTree = ({ onAction, ...context }: CollectionListTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const {
    data: { items: collections },
  } = useConnectSuspenseQuery(collectionList, { workspaceId });

  const ref = useRef<HTMLDivElement>(null);

  return (
    <CollectionListTreeContext.Provider value={{ ...context, containerRef: ref }}>
      <div className={tw`relative`} ref={ref}>
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
            return <CollectionTree collection={_} id={collectionIdCan} />;
          }}
        </Tree>
      </div>
    </CollectionListTreeContext.Provider>
  );
};

interface CollectionTreeProps {
  collection: CollectionListItem;
  id: string;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const { containerRef, showControls } = useContext(CollectionListTreeContext);

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
    onSuccess: (_) => collectionUpdateMutation.mutateAsync({ collectionId, name: _ }),
    value: collection.name,
  });

  const childItems = useMemo(
    () =>
      Array.filterMap(collectionItemListQuery.data?.items ?? [], (_) => {
        const kind = enumToString(ItemKindSchema, 'ITEM_KIND', _.kind);
        return Option.liftPredicate(_, (_) => _[kind] !== undefined);
      }),
    [collectionItemListQuery.data?.items],
  );

  return (
    <TreeItem
      childItem={mapCollectionItemTree(collectionId)}
      childItems={childItems}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      id={pipe(new TreeKey({ collectionId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      loading={collectionItemListQuery.isLoading}
      textValue={collection.name}
      wrapperOnContextMenu={onContextMenu}
    >
      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
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
          <Button className={tw`p-0.5`} variant='ghost'>
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

            <MenuItem onAction={() => void collectionDeleteMutation.mutate({ collectionId })} variant='danger'>
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
          <FolderTree collectionId={collectionId} folder={_.folder!} id={folderIdCan} parentFolderId={parentFolderId} />
        );
      }),
      Match.when({ kind: ItemKind.ENDPOINT }, (_) => {
        const endpointIdCan = Ulid.construct(_.endpoint!.endpointId).toCanonical();
        return (
          <EndpointTree collectionId={collectionId} endpoint={_.endpoint!} example={_.example!} id={endpointIdCan} />
        );
      }),
      Match.orElse(() => null),
    );

interface FolderTreeProps {
  collectionId: Collection['collectionId'];
  folder: FolderListItem;
  id: string;
  parentFolderId: Folder['folderId'] | undefined;
}

const FolderTree = ({ collectionId, folder: { folderId, ...folder }, parentFolderId }: FolderTreeProps) => {
  const { containerRef, showControls } = useContext(CollectionListTreeContext);

  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId, folderId }, { enabled });

  const childItems = useMemo(
    () =>
      Array.filterMap(collectionItemListQuery.data?.items ?? [], (_) => {
        const kind = enumToString(ItemKindSchema, 'ITEM_KIND', _.kind);
        return Option.liftPredicate(_, (_) => _[kind] !== undefined);
      }),
    [collectionItemListQuery.data?.items],
  );

  const folderCreateMutation = useConnectMutation(folderCreate);
  const folderUpdateMutation = useConnectMutation(folderUpdate);
  const folderDeleteMutation = useConnectMutation(folderDelete);

  const endpointCreateMutation = useConnectMutation(endpointCreate);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) =>
      folderUpdateMutation.mutateAsync({
        folderId,
        name: _,
        parentFolderId: parentFolderId!,
      }),
    value: folder.name,
  });

  return (
    <TreeItem
      childItem={mapCollectionItemTree(collectionId, folderId)}
      childItems={childItems}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      id={pipe(new TreeKey({ collectionId, folderId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      loading={collectionItemListQuery.isLoading}
      textValue={folder.name}
      wrapperOnContextMenu={onContextMenu}
    >
      {({ isExpanded }) => (
        <>
          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
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
              <Button className={tw`p-0.5`} variant='ghost'>
                <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
              </Button>

              <Menu {...menuProps}>
                <MenuItem onAction={() => void edit()}>Rename</MenuItem>

                <MenuItem
                  onAction={() =>
                    void endpointCreateMutation.mutate({
                      collectionId,
                      name: 'New API call',
                      parentFolderId: folderId,
                    })
                  }
                >
                  Add Request
                </MenuItem>

                <MenuItem
                  onAction={() =>
                    void folderCreateMutation.mutate({
                      collectionId,
                      name: 'New folder',
                      parentFolderId: folderId,
                    })
                  }
                >
                  Add Folder
                </MenuItem>

                <MenuItem onAction={() => void folderDeleteMutation.mutate({ folderId })} variant='danger'>
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
  collectionId: Collection['collectionId'];
  endpoint: EndpointListItem;
  example: ExampleListItem;
  id: string;
}

const EndpointTree = ({ collectionId, endpoint, example, id: endpointIdCan }: EndpointTreeProps) => {
  const { endpointId, method, name } = endpoint;
  const { exampleId, lastResponseId } = example;

  const matchRoute = useMatchRoute();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const { containerRef, navigate = false, showControls } = useContext(CollectionListTreeContext);

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
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => endpointUpdateMutation.mutateAsync({ endpointId, name: _ }),
    value: endpoint.name,
  });

  const route = {
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
  } satisfies ToOptions;

  return (
    <TreeItem
      childItem={(_) => {
        const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
        return <ExampleItem collectionId={collectionId} endpointId={endpointId} example={_} id={exampleIdCan} />;
      }}
      childItems={exampleListQuery.data?.items ?? []}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      href={navigate ? route : undefined!}
      id={pipe(new TreeKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      isActive={navigate && matchRoute(route) !== false}
      loading={exampleListQuery.isLoading}
      textValue={name}
      wrapperOnContextMenu={onContextMenu}
    >
      {method && <MethodBadge method={method} />}

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
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
          <Button className={tw`p-0.5`} variant='ghost'>
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
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export
            </MenuItem>

            <MenuItem onAction={() => void endpointDeleteMutation.mutate({ endpointId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </TreeItem>
  );
};

interface ExampleItemProps {
  collectionId: Collection['collectionId'];
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
  id: string;
}

const ExampleItem = ({ collectionId, endpointId, example, id: exampleIdCan }: ExampleItemProps) => {
  const { exampleId, lastResponseId, name } = example;

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const matchRoute = useMatchRoute();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const { containerRef, navigate = false, showControls } = useContext(CollectionListTreeContext);

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleUpdateMutation = useConnectMutation(exampleUpdate);
  const exampleDeleteMutation = useConnectMutation(exampleDelete);
  const exampleDuplicateMutation = useConnectMutation(exampleDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => exampleUpdateMutation.mutateAsync({ exampleId, name: _ }),
    value: name,
  });

  const route = {
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
  } satisfies ToOptions;

  return (
    <TreeItem
      href={navigate ? route : undefined!}
      id={pipe(new TreeKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      isActive={navigate && matchRoute(route) !== false}
      textValue={name}
      wrapperOnContextMenu={onContextMenu}
    >
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
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
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => void exampleDuplicateMutation.mutate({ exampleId })}>Duplicate</MenuItem>

            <MenuItem
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export
            </MenuItem>

            <MenuItem onAction={() => void exampleDeleteMutation.mutate({ exampleId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </TreeItem>
  );
};
