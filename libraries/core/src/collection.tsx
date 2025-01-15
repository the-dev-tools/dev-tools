import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, ToOptions, useMatchRoute } from '@tanstack/react-router';
import { Effect, Match, pipe, Runtime, Schema } from 'effect';
import { Ulid } from 'id128';
import { createContext, useContext, useMemo, useRef, useState } from 'react';
import { Form, MenuTrigger, Text, UNSTABLE_Tree as Tree } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal, FiRotateCw } from 'react-icons/fi';
import { MdLightbulbOutline } from 'react-icons/md';

import { Endpoint, EndpointListItem } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import {
  endpointCreate,
  endpointDelete,
  endpointDuplicate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleListItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleCreate,
  exampleDelete,
  exampleDuplicate,
  exampleList,
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
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Popover } from '@the-dev-tools/ui/popover';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';

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
}

const CollectionListTreeContext = createContext({} as CollectionListTreeContext);

class TreeKey extends Schema.Class<TreeKey>('CollectionListTreeKey')({
  collectionId: pipe(Schema.Uint8Array, Schema.optional),
  folderId: pipe(Schema.Uint8Array, Schema.optional),
  endpointId: pipe(Schema.Uint8Array, Schema.optional),
  exampleId: pipe(Schema.Uint8Array, Schema.optional),
}) {}

interface CollectionListTreeProps extends CollectionListTreeContext {
  onAction?: (key: TreeKey) => void;
}

export const CollectionListTree = ({ onAction, ...context }: CollectionListTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const collectionListQuery = useConnectQuery(collectionList, { workspaceId });

  if (!collectionListQuery.isSuccess) return null;
  const collections = collectionListQuery.data.items;

  return (
    <CollectionListTreeContext.Provider value={context}>
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
    </CollectionListTreeContext.Provider>
  );
};

interface CollectionTreeProps {
  id: string;
  collection: CollectionListItem;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { runtime } = workspaceRoute.useRouteContext();

  const { showControls } = useContext(CollectionListTreeContext);

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId }, { enabled });
  const collectionDeleteMutation = useConnectMutation(collectionDelete);
  const collectionUpdateMutation = useConnectMutation(collectionUpdate);

  const folderCreateMutation = useConnectMutation(folderCreate);
  const endpointCreateMutation = useConnectMutation(endpointCreate);

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

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
    >
      {collectionItemListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <Text ref={triggerRef} className='flex-1 truncate'>
        {collection.name}
      </Text>

      {showControls && (
        <>
          <MenuTrigger>
            <Button variant='ghost' className={tw`p-0.5`}>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu>
              <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

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

          <Popover
            triggerRef={triggerRef}
            isOpen={isRenaming}
            onOpenChange={setIsRenaming}
            dialogAria-label='Rename collection'
          >
            <Form
              className='flex flex-1 items-center gap-2'
              onSubmit={(event) =>
                Effect.gen(function* () {
                  event.preventDefault();

                  const { name } = yield* pipe(
                    new FormData(event.currentTarget),
                    Object.fromEntries,
                    Schema.decode(Schema.Struct({ name: Schema.String })),
                  );

                  collectionUpdateMutation.mutate({ workspaceId, collectionId, name });

                  setIsRenaming(false);
                }).pipe(Runtime.runPromise(runtime))
              }
            >
              <TextField
                name='name'
                defaultValue={collection.name}
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                label='New name:'
                className={tw`contents`}
                labelClassName={tw`text-nowrap`}
                inputClassName={tw`w-full bg-transparent`}
              />

              <Button type='submit'>Save</Button>
            </Form>
          </Popover>
        </>
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

const FolderTree = ({ collectionId, parentFolderId, folder }: FolderTreeProps) => {
  const { runtime } = workspaceRoute.useRouteContext();

  const { showControls } = useContext(CollectionListTreeContext);

  const { folderId } = folder;
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

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      id={pipe(new TreeKey({ collectionId, folderId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      textValue={folder.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId, folderId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
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

          <Text ref={triggerRef} className='flex-1 truncate'>
            {folder.name}
          </Text>

          {showControls && (
            <>
              <MenuTrigger>
                <Button variant='ghost' className={tw`p-0.5`}>
                  <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
                </Button>

                <Menu>
                  <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

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

              <Popover
                triggerRef={triggerRef}
                isOpen={isRenaming}
                onOpenChange={setIsRenaming}
                dialogAria-label='Rename folder'
              >
                <Form
                  className='flex flex-1 items-center gap-2'
                  onSubmit={(event) =>
                    Effect.gen(function* () {
                      event.preventDefault();

                      const { name } = yield* pipe(
                        new FormData(event.currentTarget),
                        Object.fromEntries,
                        Schema.decode(Schema.Struct({ name: Schema.String })),
                      );

                      folderUpdateMutation.mutate({
                        collectionId,
                        folderId,
                        name,
                        parentFolderId: parentFolderId!,
                      });

                      setIsRenaming(false);
                    }).pipe(Runtime.runPromise(runtime))
                  }
                >
                  <TextField
                    name='name'
                    defaultValue={folder.name}
                    // eslint-disable-next-line jsx-a11y/no-autofocus
                    autoFocus
                    label='New name:'
                    className={tw`contents`}
                    labelClassName={tw`text-nowrap`}
                    inputClassName={tw`w-full bg-transparent`}
                  />

                  <Button type='submit'>Save</Button>
                </Form>
              </Popover>
            </>
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

  const { navigate = false, showControls } = useContext(CollectionListTreeContext);

  const exampleIdCan = Ulid.construct(exampleId).toCanonical();
  const lastResponseIdCan = lastResponseId.length > 0 ? Ulid.construct(lastResponseId).toCanonical() : undefined;

  const [enabled, setEnabled] = useState(false);

  const exampleListQuery = useConnectQuery(exampleList, { endpointId }, { enabled });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const exampleCreateMutation = useConnectMutation(exampleCreate);
  const endpointDeleteMutation = useConnectMutation(endpointDelete);
  const endpointDuplicateMutation = useConnectMutation(endpointDuplicate, {
    onSuccess: invalidateCollectionListQuery,
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
    >
      {exampleListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <MethodBadge method={method} />

      <Text className='flex-1 truncate'>{name}</Text>

      {showControls && (
        <>
          <MenuTrigger>
            <Button variant='ghost' className={tw`p-0.5`}>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu>
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
        </>
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
  const lastResponseIdCan = lastResponseId.length > 0 ? Ulid.construct(lastResponseId).toCanonical() : undefined;

  const matchRoute = useMatchRoute();

  const { navigate = false, showControls } = useContext(CollectionListTreeContext);

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleDeleteMutation = useConnectMutation(exampleDelete);
  const exampleDuplicateMutation = useConnectMutation(exampleDuplicate, {
    onSuccess: invalidateCollectionListQuery,
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
    >
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text className='flex-1 truncate'>{name}</Text>

      {showControls && (
        <MenuTrigger>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu>
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
