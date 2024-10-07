import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Outlet, redirect, useMatch } from '@tanstack/react-router';
import { Effect, Match, pipe, Struct } from 'effect';
import { useRef, useState } from 'react';
import { FileTrigger, Form, MenuTrigger, Text } from 'react-aria-components';
import { LuFolder, LuImport, LuMoreHorizontal, LuPlus } from 'react-icons/lu';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { CollectionMeta } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import {
  createCollection,
  deleteCollection,
  importPostman,
  listCollections,
  updateCollection,
} from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { ApiCallMeta } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { createApiCall, deleteApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { ApiExampleMeta } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createExample,
  deleteExample,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { FolderMeta, ItemMeta } from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder_pb';
import {
  createFolder,
  deleteFolder,
  updateFolder,
} from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder-ItemFolderService_connectquery';
import { getWorkspace } from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';
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

export const Route = createFileRoute('/_authorized/workspace/$workspaceId')({
  component: Layout,
  loader: async ({ params: { workspaceId } }) => {
    const options = createQueryOptions(getWorkspace, { id: workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
  },
});

const useInvalidateList = () => {
  const { workspaceId } = Route.useParams();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });
  return () => queryClient.invalidateQueries(listQueryOptions);
};

const useCreateFolderMutation = () => {
  const invalidateList = useInvalidateList();
  return useConnectMutation(createFolder, { onSuccess: invalidateList });
};

const useCreateApiCallMutation = () => {
  const invalidateList = useInvalidateList();
  return useConnectMutation(createApiCall, { onSuccess: invalidateList });
};

function Layout() {
  const { workspaceId } = Route.useParams();

  const query = useConnectQuery(getWorkspace, { id: workspaceId });
  if (!query.isSuccess) return;
  const { workspace } = query.data;

  return (
    <DashboardLayout
      leftChildren={
        <MenuTrigger>
          <Button kind='placeholder' className='bg-transparent text-white' variant='placeholder'>
            {workspace!.name}
          </Button>
          <Menu>
            <MenuItem href={{ to: '/workspace/$workspaceId', params: { workspaceId } }}>Home</MenuItem>
            <MenuItem href={{ to: '/workspace/$workspaceId/members', params: { workspaceId } }}>Members</MenuItem>
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
  const { workspaceId } = Route.useParams();

  const collectionsQuery = useConnectQuery(listCollections, { workspaceId });

  const invalidateList = useInvalidateList();
  const createCollectionMutation = useConnectMutation(createCollection, { onSuccess: invalidateList });

  if (!collectionsQuery.isSuccess) return null;
  const metaCollections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h3 className='uppercase'>Collections</h3>
      <div className='flex justify-between gap-2'>
        <Button
          kind='placeholder'
          variant='placeholder'
          onPress={() => void createCollectionMutation.mutate({ workspaceId, name: 'New collection' })}
          className='flex-1 font-medium'
        >
          <LuPlus />
          New
        </Button>
        <ImportPostman />
      </div>
      <Tree aria-label='Collections' items={metaCollections}>
        {(_) => <CollectionTree id={_.id} meta={_} />}
      </Tree>
    </>
  );
};

interface CollectionTreeProps {
  id: string;
  meta: CollectionMeta;
}

const CollectionTree = ({ meta }: CollectionTreeProps) => {
  const invalidateList = useInvalidateList();
  const deleteMutation = useConnectMutation(deleteCollection, { onSuccess: invalidateList });
  const updateMutation = useConnectMutation(updateCollection, { onSuccess: invalidateList });
  const createFolderMutation = useCreateFolderMutation();
  const createApiCallMutation = useCreateApiCallMutation();

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={meta.items}
      childItem={(_) => <FolderItemTree id={_.meta.value!.id} collectionId={meta.id} item={_} />}
    >
      <Text ref={triggerRef} className='flex-1 truncate'>
        {meta.name}
      </Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem
            onAction={() =>
              void createApiCallMutation.mutate({ data: { collectionId: meta.id, meta: { name: 'New API call' } } })
            }
          >
            Add Request
          </MenuItem>

          <MenuItem
            onAction={() =>
              void createFolderMutation.mutate({ folder: { collectionId: meta.id, meta: { name: 'New folder' } } })
            }
          >
            Add Folder
          </MenuItem>

          <MenuItem variant='danger' onAction={() => void deleteMutation.mutate({ id: meta.id })}>
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

              updateMutation.mutate({ id: meta.id, name });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise)
          }
        >
          <TextField
            name='name'
            defaultValue={meta.name}
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

interface FolderItemTreeProps {
  id: string;
  collectionId: string;
  item: ItemMeta;
}

const FolderItemTree = ({ collectionId, item }: FolderItemTreeProps) =>
  pipe(
    item.meta,
    Match.value,
    Match.when({ case: 'folderMeta' }, (_) => <FolderTree collectionId={collectionId} meta={_.value} />),
    Match.when({ case: 'apiCallMeta' }, (_) => <ApiCallTree meta={_.value} />),
    Match.orElse(() => null),
  );

interface FolderTreeProps {
  collectionId: string;
  meta: FolderMeta;
}

const FolderTree = ({ collectionId, meta }: FolderTreeProps) => {
  const invalidateList = useInvalidateList();
  const deleteMutation = useConnectMutation(deleteFolder, { onSuccess: invalidateList });
  const updateMutation = useConnectMutation(updateFolder, { onSuccess: invalidateList });
  const createFolderMutation = useCreateFolderMutation();
  const createApiCallMutation = useCreateApiCallMutation();

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={meta.items}
      childItem={(_) => <FolderItemTree id={_.meta.value!.id} collectionId={collectionId} item={_} />}
    >
      <LuFolder />

      <Text ref={triggerRef} className='flex-1 truncate'>
        {meta.name}
      </Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem
            onAction={() =>
              void createApiCallMutation.mutate({
                data: { collectionId, parentId: meta.id, meta: { name: 'New API call' } },
              })
            }
          >
            Add Request
          </MenuItem>

          <MenuItem
            onAction={() =>
              void createFolderMutation.mutate({
                folder: { collectionId, parentId: meta.id, meta: { name: 'New folder' } },
              })
            }
          >
            Add Folder
          </MenuItem>

          <MenuItem variant='danger' onAction={() => void deleteMutation.mutate({ id: meta.id })}>
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

              updateMutation.mutate({
                folder: { meta: Struct.evolve(meta, { name: () => name }) },
              });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise)
          }
        >
          <TextField
            name='name'
            defaultValue={meta.name}
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

interface ApiCallTreeProps {
  meta: ApiCallMeta;
}

const ApiCallTree = ({ meta }: ApiCallTreeProps) => {
  const match = useMatch({ strict: false });

  const invalidateList = useInvalidateList();
  const deleteMutation = useConnectMutation(deleteApiCall, { onSuccess: invalidateList });
  const createExampleMutation = useConnectMutation(createExample, { onSuccess: invalidateList });

  return (
    <TreeItem
      textValue={meta.name}
      href={{
        from: Route.fullPath,
        to: '/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId',
        params: { apiCallId: meta.id, exampleId: meta.defaultExampleId },
      }}
      wrapperIsSelected={match.params.exampleId === meta.defaultExampleId}
      childItems={meta.examples}
      childItem={(_) => <ApiExampleItem apiCallId={meta.id} meta={_} />}
    >
      {!meta.examples.length && <div />}

      <div className='text-sm font-bold'>{meta.method}</div>

      <Text className='flex-1 truncate'>{meta.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem
            onAction={() =>
              void createExampleMutation.mutate({ itemApiId: meta.id, example: { meta: { name: 'New Example' } } })
            }
          >
            Add Example
          </MenuItem>

          <MenuItem variant='danger' onAction={() => void deleteMutation.mutate({ id: meta.id })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

interface ApiExampleItemProps {
  apiCallId: string;
  meta: ApiExampleMeta;
}

const ApiExampleItem = ({ apiCallId, meta }: ApiExampleItemProps) => {
  const match = useMatch({ strict: false });

  const invalidateList = useInvalidateList();
  const deleteMutation = useConnectMutation(deleteExample, { onSuccess: invalidateList });

  return (
    <TreeItem
      textValue={meta.name}
      href={{
        from: Route.fullPath,
        to: '/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId',
        params: { apiCallId: apiCallId, exampleId: meta.id },
      }}
      wrapperIsSelected={match.params.exampleId === meta.id}
    >
      <div />

      <Text className='flex-1 truncate'>{meta.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem variant='danger' onAction={() => void deleteMutation.mutate({ id: meta.id })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

const ImportPostman = () => {
  const { workspaceId } = Route.useParams();

  const invalidateList = useInvalidateList();
  const createMutation = useConnectMutation(importPostman, { onSuccess: invalidateList });

  return (
    <FileTrigger
      onSelect={async (_) => {
        const file = _?.item(0);
        if (!file) return;
        const data = new Uint8Array(await file.arrayBuffer());
        createMutation.mutate({ workspaceId, name: file.name, data });
      }}
    >
      <Button kind='placeholder' variant='placeholder' className='flex-1 font-medium'>
        <LuImport />
        Import
      </Button>
    </FileTrigger>
  );
};
