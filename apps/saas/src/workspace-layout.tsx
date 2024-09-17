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
import { deleteApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
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
import { queryClient, Runtime, transport } from './runtime';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId')({
  component: Layout,
  loader: async ({ params: { workspaceId } }) => {
    const options = createQueryOptions(getWorkspace, { id: workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
  },
});

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
        <Panel
          className='flex flex-col gap-2 p-2'
          style={{ overflowY: 'auto' }}
          defaultSize={20}
          minSize={10}
          maxSize={40}
        >
          <h2 className='uppercase'>Overview</h2>
          <CollectionsTree />
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

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createCollectionMutation = useConnectMutation(createCollection);
  const collectionsQuery = useConnectQuery(listCollections, { workspaceId });

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  if (!collectionsQuery.isSuccess) return null;
  const metaCollections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h3 className='uppercase'>Collections</h3>
      <div className='flex justify-between gap-2'>
        <Button
          kind='placeholder'
          variant='placeholder'
          onPress={async () => {
            await createCollectionMutation.mutateAsync({ workspaceId, name: 'New collection' });
            await queryClient.invalidateQueries(listQueryOptions);
          }}
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
  const { workspaceId } = Route.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useConnectMutation(deleteCollection);
  const updateMutation = useConnectMutation(updateCollection);
  const createFolderMutation = useConnectMutation(createFolder);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={meta.items}
      childItem={(_) => <FolderItemTree id={_.meta.value!.id} item={_} />}
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
            onAction={async () => {
              await deleteMutation.mutateAsync({ id: meta.id });
              await queryClient.invalidateQueries(listQueryOptions);
            }}
          >
            Delete
          </MenuItem>

          <MenuItem
            onAction={async () => {
              await createFolderMutation.mutateAsync({ collectionId: meta.id, name: 'New folder' });
              await queryClient.invalidateQueries(listQueryOptions);
            }}
          >
            Create folder
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

              yield* Effect.tryPromise(() => updateMutation.mutateAsync({ id: meta.id, name }));

              yield* Effect.tryPromise(() => queryClient.invalidateQueries(listQueryOptions));

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
  item: ItemMeta;
}

const FolderItemTree = ({ item }: FolderItemTreeProps) =>
  pipe(
    item.meta,
    Match.value,
    Match.when({ case: 'folderMeta' }, (_) => <FolderTree meta={_.value} />),
    Match.when({ case: 'apiCallMeta' }, (_) => <ApiCallTree meta={_.value} />),
    Match.orElse(() => null),
  );

interface FolderTreeProps {
  meta: FolderMeta;
}

const FolderTree = ({ meta }: FolderTreeProps) => {
  const { workspaceId } = Route.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useConnectMutation(deleteFolder);
  const updateMutation = useConnectMutation(updateFolder);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={meta.items}
      childItem={(_) => <FolderItemTree id={_.meta.value!.id} item={_} />}
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
            onAction={async () => {
              await deleteMutation.mutateAsync({ id: meta.id });
              await queryClient.invalidateQueries(listQueryOptions);
            }}
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

              yield* Effect.tryPromise(() =>
                updateMutation.mutateAsync({
                  folder: { meta: Struct.evolve(meta, { name: () => name }) },
                }),
              );

              yield* Effect.tryPromise(() => queryClient.invalidateQueries(listQueryOptions));

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
  const transport = useTransport();
  const queryClient = useQueryClient();

  const match = useMatch({ strict: false });

  const { workspaceId } = Route.useParams();

  const deleteMutation = useConnectMutation(deleteApiCall);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  return (
    <TreeItem
      textValue={meta.name}
      href={{ to: '/workspace/$workspaceId/api-call/$apiCallId', params: { workspaceId, apiCallId: meta.id } }}
      wrapperIsSelected={match.params.apiCallId === meta.id}
    >
      <div />

      <div className='text-sm font-bold'>{meta.method}</div>

      <Text className='flex-1 truncate'>{meta.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem
            onAction={async () => {
              await deleteMutation.mutateAsync({ id: meta.id });
              await queryClient.invalidateQueries(listQueryOptions);
            }}
          >
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

const ImportPostman = () => {
  const { workspaceId } = Route.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createMutation = useConnectMutation(importPostman);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  return (
    <FileTrigger
      onSelect={async (_) => {
        const file = _?.item(0);
        if (!file) return;
        await createMutation.mutateAsync({
          workspaceId,
          name: file.name,
          data: new Uint8Array(await file.arrayBuffer()),
        });
        await queryClient.invalidateQueries(listQueryOptions);
      }}
    >
      <Button kind='placeholder' variant='placeholder' className='flex-1 font-medium'>
        <LuImport />
        Import
      </Button>
    </FileTrigger>
  );
};
