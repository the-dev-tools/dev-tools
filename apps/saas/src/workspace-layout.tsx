import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Outlet, redirect, useMatch } from '@tanstack/react-router';
import { Effect, Match, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { useMemo, useRef, useState } from 'react';
import { FileTrigger, Form, MenuTrigger, Text } from 'react-aria-components';
import { LuFolder, LuImport, LuLoader, LuMoreHorizontal, LuPlus } from 'react-icons/lu';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { useCreateMutation } from '@the-dev-tools/api/query';
import { EndpointListItem } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
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
import { FolderListItem } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import {
  folderCreate,
  folderDelete,
  folderUpdate,
} from '@the-dev-tools/spec/collection/item/folder/v1/folder-FolderService_connectquery';
import { CollectionItem, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import {
  collectionCreate,
  collectionDelete,
  collectionImportPostman,
  collectionList,
  collectionUpdate,
} from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { workspaceGet } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Popover } from '@the-dev-tools/ui/popover';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import { Tree, TreeItem } from '@the-dev-tools/ui/tree';

import { DashboardLayout } from './authorized';
import { EnvironmentsWidget } from './environment';
import { queryClient, Runtime, transport } from './runtime';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan')({
  component: Layout,
  pendingComponent: () => 'Loading workspace...',
  loader: async ({ params: { workspaceIdCan } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    const options = createQueryOptions(workspaceGet, { workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
    return { workspaceId };
  },
});

const useInvalidateCollectionListQuery = () => {
  const { workspaceId } = Route.useLoaderData();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const collectionListQueryOptions = createQueryOptions(collectionList, { workspaceId }, { transport });
  return () => queryClient.invalidateQueries(collectionListQueryOptions);
};

function Layout() {
  const { workspaceId } = Route.useLoaderData();
  const { workspaceIdCan } = Route.useParams();

  const workspaceGetQuery = useConnectQuery(workspaceGet, { workspaceId });
  if (!workspaceGetQuery.isSuccess) return;
  const workspace = workspaceGetQuery.data;

  return (
    <DashboardLayout
      leftChildren={
        <MenuTrigger>
          <Button kind='placeholder' className='bg-transparent text-white' variant='placeholder'>
            {workspace.name}
          </Button>
          <Menu>
            <MenuItem
              href={{
                to: '/workspace/$workspaceIdCan',
                params: { workspaceIdCan },
              }}
            >
              Home
            </MenuItem>
            <MenuItem
              href={{
                to: '/workspace/$workspaceIdCan/members',
                params: { workspaceIdCan },
              }}
            >
              Members
            </MenuItem>
          </Menu>
        </MenuTrigger>
      }
    >
      <PanelGroup direction='horizontal'>
        <Panel className='flex flex-col' style={{ overflowY: 'auto' }} defaultSize={20} minSize={10} maxSize={40}>
          <EnvironmentsWidget />

          <div className='flex flex-col gap-2 p-2'>
            <h2 className='uppercase'>Overview</h2>

            <CollectionsTree />
          </div>
        </Panel>
        <PanelResizeHandle direction='horizontal' />
        <Panel className='h-full !overflow-auto'>
          <Outlet />
        </Panel>
      </PanelGroup>
    </DashboardLayout>
  );
}

const CollectionsTree = () => {
  const { workspaceId } = Route.useLoaderData();

  const collectionListInput = { workspaceId };
  const collectionListQuery = useConnectQuery(collectionList, collectionListInput);

  const collectionCreateMutation = useCreateMutation(collectionCreate, {
    key: 'collectionId',
    listQuery: collectionList,
    listInput: collectionListInput,
  });

  if (!collectionListQuery.isSuccess) return null;
  const collections = collectionListQuery.data.items;

  return (
    <>
      <h3 className='uppercase'>Collections</h3>
      <div className='flex justify-between gap-2'>
        <Button
          kind='placeholder'
          variant='placeholder'
          onPress={() =>
            void collectionCreateMutation.mutate({
              workspaceId,
              name: 'New collection',
            })
          }
          className='flex-1 font-medium'
        >
          <LuPlus />
          New
        </Button>
        <ImportPostman />
      </div>
      <Tree aria-label='Collections' items={collections}>
        {(_) => {
          const collectionIdCan = Ulid.construct(_.collectionId).toCanonical();
          return <CollectionTree id={collectionIdCan} collection={_} />;
        }}
      </Tree>
    </>
  );
};

interface CollectionTreeProps {
  id: string;
  collection: CollectionListItem;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListInput = { collectionId };
  const collectionItemListQuery = useConnectQuery(collectionItemList, collectionItemListInput, { enabled });

  const collectionDeleteMutation = useConnectMutation(collectionDelete, {
    onSuccess: invalidateCollectionListQuery,
  });
  const collectionUpdateMutation = useConnectMutation(collectionUpdate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const folderCreateMutation = useCreateMutation(folderCreate, {
    // TODO: fix incorrect key id
    key: 'folderId',
    listQuery: collectionItemList,
    listInput: collectionItemListInput,
  });

  const endpointCreateMutation = useCreateMutation(endpointCreate, {
    // TODO: fix incorrect key id
    key: 'endpointId',
    listQuery: collectionItemList,
    listInput: collectionItemListInput,
  });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  return (
    <TreeItem
      textValue={collection.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {collectionItemListQuery.isLoading && (
        <Button kind='placeholder' variant='placeholder ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <Text ref={triggerRef} className='flex-1 truncate'>
        {collection.name}
      </Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem
            onAction={() =>
              void endpointCreateMutation.mutate({
                collectionId,
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
                name: 'New folder',
              })
            }
          >
            Add Folder
          </MenuItem>

          <MenuItem variant='danger' onAction={() => void collectionDeleteMutation.mutate({ collectionId })}>
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

              collectionUpdateMutation.mutate({ collectionId, name });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise)
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

          <Button kind='placeholder' variant='placeholder' type='submit'>
            Save
          </Button>
        </Form>
      </Popover>
    </TreeItem>
  );
};

const mapCollectionItemTree = (collectionId: Collection['collectionId']) => (item: CollectionItem) =>
  pipe(
    Match.value(item),
    Match.when({ kind: ItemKind.FOLDER }, (_) => {
      const folderIdCan = Ulid.construct(_.folder!.folderId).toCanonical();
      return <FolderTree id={folderIdCan} collectionId={collectionId} folder={_.folder!} />;
    }),
    Match.when({ kind: ItemKind.ENDPOINT }, (_) => {
      const endpointIdCan = Ulid.construct(_.endpoint!.endpointId).toCanonical();
      return <EndpointTree id={endpointIdCan} endpoint={_.endpoint!} example={_.example!} />;
    }),
    Match.orElse(() => null),
  );

interface FolderTreeProps {
  id: string;
  collectionId: Collection['collectionId'];
  folder: FolderListItem;
}

const FolderTree = ({ collectionId, folder }: FolderTreeProps) => {
  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const folderDeleteMutation = useConnectMutation(folderDelete, {
    onSuccess: invalidateCollectionListQuery,
  });
  const folderUpdateMutation = useConnectMutation(folderUpdate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const { folderId } = folder;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListInput = { collectionId, folderId };
  const collectionItemListQuery = useConnectQuery(collectionItemList, collectionItemListInput, { enabled });

  const folderCreateMutation = useCreateMutation(folderCreate, {
    // TODO: fix incorrect key id
    key: 'folderId',
    listQuery: collectionItemList,
    listInput: collectionItemListInput,
  });

  const endpointCreateMutation = useCreateMutation(endpointCreate, {
    // TODO: fix incorrect key id
    key: 'endpointId',
    listQuery: collectionItemList,
    listInput: collectionItemListInput,
  });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  return (
    <TreeItem
      textValue={folder.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {collectionItemListQuery.isLoading && (
        <Button kind='placeholder' variant='placeholder ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <LuFolder />

      <Text ref={triggerRef} className='flex-1 truncate'>
        {folder.name}
      </Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
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

          <MenuItem variant='danger' onAction={() => void folderDeleteMutation.mutate({ folderId })}>
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

              folderUpdateMutation.mutate({ folderId, name });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise)
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

          <Button kind='placeholder' variant='placeholder' type='submit'>
            Save
          </Button>
        </Form>
      </Popover>
    </TreeItem>
  );
};

interface EndpointTreeProps {
  id: string;
  endpoint: EndpointListItem;
  example: ExampleListItem;
}

const EndpointTree = ({ id: endpointIdCan, endpoint, example }: EndpointTreeProps) => {
  const match = useMatch({ strict: false });

  const exampleIdCan = Ulid.construct(example.exampleId).toCanonical();
  const { endpointId, method } = endpoint;

  const [enabled, setEnabled] = useState(false);

  const exampleListInput = { endpointId };
  const exampleListQuery = useConnectQuery(exampleList, exampleListInput, { enabled });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const endpointDeleteMutation = useConnectMutation(endpointDelete, {
    onSuccess: invalidateCollectionListQuery,
  });
  const endpointDuplicateMutation = useConnectMutation(endpointDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const exampleCreateMutation = useCreateMutation(exampleCreate, {
    key: 'exampleId',
    listQuery: exampleList,
    listInput: exampleListInput,
  });

  return (
    <TreeItem
      textValue={endpoint.name}
      href={{
        from: Route.fullPath,
        to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
        params: { endpointIdCan, exampleIdCan },
      }}
      wrapperIsSelected={match.params.exampleIdCan === exampleIdCan}
      childItems={exampleListQuery.data?.items ?? []}
      childItem={(_) => {
        const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
        return <ExampleItem id={exampleIdCan} endpointIdCan={endpointIdCan} example={_} />;
      }}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {exampleListQuery.isLoading && (
        <Button kind='placeholder' variant='placeholder ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <div className='text-sm font-bold'>{method}</div>

      <Text className='flex-1 truncate'>{endpoint.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
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

          <MenuItem variant='danger' onAction={() => void endpointDeleteMutation.mutate({ endpointId })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

interface ExampleItemProps {
  id: string;
  endpointIdCan: string;
  example: ExampleListItem;
}

const ExampleItem = ({ id: exampleIdCan, endpointIdCan, example }: ExampleItemProps) => {
  const match = useMatch({ strict: false });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleDeleteMutation = useConnectMutation(exampleDelete, {
    onSuccess: invalidateCollectionListQuery,
  });
  const exampleDuplicateMutation = useConnectMutation(exampleDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const { exampleId } = example;

  return (
    <TreeItem
      textValue={example.name}
      href={{
        from: Route.fullPath,
        to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
        params: { endpointIdCan, exampleIdCan },
      }}
      wrapperIsSelected={match.params.exampleIdCan === exampleIdCan}
    >
      <div />

      <Text className='flex-1 truncate'>{example.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem onAction={() => void exampleDuplicateMutation.mutate({ exampleId })}>Duplicate</MenuItem>

          <MenuItem variant='danger' onAction={() => void exampleDeleteMutation.mutate({ exampleId })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

const ImportPostman = () => {
  const { workspaceId } = Route.useLoaderData();

  const collectionImportPostmanMutation = useCreateMutation(collectionImportPostman, {
    key: 'collectionId',
    listQuery: collectionList,
    listInput: { workspaceId },
  });

  return (
    <FileTrigger
      onSelect={async (_) => {
        const file = _?.item(0);
        if (!file) return;
        const data = new Uint8Array(await file.arrayBuffer());
        collectionImportPostmanMutation.mutate({ workspaceId, name: file.name, data });
      }}
    >
      <Button kind='placeholder' variant='placeholder' className='flex-1 font-medium'>
        <LuImport />
        Import
      </Button>
    </FileTrigger>
  );
};
