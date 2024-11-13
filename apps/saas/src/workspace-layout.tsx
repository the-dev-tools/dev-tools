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
import { FileTrigger, Form, MenuTrigger, Text, UNSTABLE_Tree as Tree } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { LuFolder, LuLoader } from 'react-icons/lu';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { useSpecMutation } from '@the-dev-tools/api/query';
import {
  collectionCreateSpec,
  collectionDeleteSpec,
  collectionImportPostmanSpec,
  collectionUpdateSpec,
} from '@the-dev-tools/api/spec/collection';
import { endpointCreateSpec, endpointDeleteSpec } from '@the-dev-tools/api/spec/collection/item/endpoint';
import { exampleCreateSpec, exampleDeleteSpec } from '@the-dev-tools/api/spec/collection/item/example';
import { folderCreateSpec, folderDeleteSpec, folderUpdateSpec } from '@the-dev-tools/api/spec/collection/item/folder';
import { Endpoint, EndpointListItem } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import { endpointDuplicate } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleListItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleDuplicate,
  exampleList,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { Folder, FolderListItem } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import { CollectionItem, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import { collectionList } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { workspaceGet } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon, FileImportIcon, FlowsIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Popover } from '@the-dev-tools/ui/popover';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';

import { DashboardLayout } from './authorized';
import { EnvironmentsWidget } from './environment';
import { Runtime } from './runtime';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan')({
  component: Layout,
  pendingComponent: () => 'Loading workspace...',
  loader: async ({ params: { workspaceIdCan }, context: { transport, queryClient } }) => {
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
      navbar={
        <>
          <MenuTrigger>
            <Button className='bg-transparent text-white'>{workspace.name}</Button>
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
          <div className='flex-1' />
        </>
      }
    >
      <PanelGroup direction='horizontal'>
        <Panel
          className={tw`flex flex-col bg-slate-50`}
          style={{ overflowY: 'auto' }}
          defaultSize={20}
          minSize={10}
          maxSize={40}
        >
          <EnvironmentsWidget />

          <div className={tw`flex flex-col gap-2 p-1.5`}>
            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <OverviewIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>Overview</h2>
            </div>

            <CollectionsTree />

            {/* TODO: implement */}
            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <FlowsIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`flex-1 text-md font-semibold leading-5 tracking-tight text-slate-800`}>Flows</h2>

              <Button className={tw`p-0.5`} variant='ghost'>
                <FileImportIcon className={tw`size-4 text-slate-500`} />
              </Button>

              <Button className={tw`bg-slate-200 p-0.5`} variant='ghost'>
                <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
              </Button>
            </div>
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

  const collectionListQuery = useConnectQuery(collectionList, { workspaceId });
  const collectionCreateMutation = useSpecMutation(collectionCreateSpec);
  const collectionImportPostmanMutation = useSpecMutation(collectionImportPostmanSpec);

  if (!collectionListQuery.isSuccess) return null;
  const collections = collectionListQuery.data.items;

  return (
    <>
      <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
        <CollectionIcon className={tw`size-5 text-slate-500`} />
        <h2 className={tw`flex-1 text-md font-semibold leading-5 tracking-tight text-slate-800`}>Collections</h2>

        <FileTrigger
          onSelect={async (_) => {
            const file = _?.item(0);
            if (!file) return;
            const data = new Uint8Array(await file.arrayBuffer());
            collectionImportPostmanMutation.mutate({ workspaceId, name: file.name, data });
          }}
        >
          <Button className={tw`p-0.5`} variant='ghost'>
            <FileImportIcon className={tw`size-4 text-slate-500`} />
          </Button>
        </FileTrigger>

        <Button
          className={tw`bg-slate-200 p-0.5`}
          variant='ghost'
          onPress={() => void collectionCreateMutation.mutate({ workspaceId, name: 'New collection' })}
        >
          <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>
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
  const { workspaceId } = Route.useLoaderData();

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId }, { enabled });
  const collectionDeleteMutation = useSpecMutation(collectionDeleteSpec);
  const collectionUpdateMutation = useSpecMutation(collectionUpdateSpec);

  const folderCreateMutation = useSpecMutation(folderCreateSpec);
  const endpointCreateMutation = useSpecMutation(endpointCreateSpec);

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
        <Button variant='ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <Text ref={triggerRef} className='flex-1 truncate'>
        {collection.name}
      </Text>

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

          <Button type='submit'>Save</Button>
        </Form>
      </Popover>
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
  const { folderId } = folder;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId, folderId }, { enabled });

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  const folderCreateMutation = useSpecMutation(folderCreateSpec);
  const folderUpdateMutation = useSpecMutation(folderUpdateSpec);
  const folderDeleteMutation = useSpecMutation(folderDeleteSpec);

  const endpointCreateMutation = useSpecMutation(endpointCreateSpec);

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={folder.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId, folderId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {collectionItemListQuery.isLoading && (
        <Button variant='ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <LuFolder />

      <Text ref={triggerRef} className='flex-1 truncate'>
        {folder.name}
      </Text>

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

          <Button type='submit'>Save</Button>
        </Form>
      </Popover>
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
  const match = useMatch({ strict: false });

  const exampleIdCan = Ulid.construct(example.exampleId).toCanonical();
  const { endpointId, method } = endpoint;

  const [enabled, setEnabled] = useState(false);

  const exampleListQuery = useConnectQuery(exampleList, { endpointId }, { enabled });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const exampleCreateMutation = useSpecMutation(exampleCreateSpec);
  const endpointDeleteMutation = useSpecMutation(endpointDeleteSpec);
  const endpointDuplicateMutation = useConnectMutation(endpointDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  return (
    <TreeItem
      textValue={endpoint.name}
      href={{
        from: Route.fullPath,
        to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
        params: { endpointIdCan, exampleIdCan },
      }}
      isActive={match.params.exampleIdCan === exampleIdCan}
      childItems={exampleListQuery.data?.items ?? []}
      childItem={(_) => {
        const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
        return <ExampleItem id={exampleIdCan} endpointId={endpointId} example={_} />;
      }}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {exampleListQuery.isLoading && (
        <Button variant='ghost' isDisabled>
          <LuLoader className='animate-spin' />
        </Button>
      )}

      <div className='text-sm font-bold'>{method}</div>

      <Text className='flex-1 truncate'>{endpoint.name}</Text>

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
    </TreeItem>
  );
};

interface ExampleItemProps {
  id: string;
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
}

const ExampleItem = ({ id: exampleIdCan, endpointId, example }: ExampleItemProps) => {
  const match = useMatch({ strict: false });

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleDeleteMutation = useSpecMutation(exampleDeleteSpec);
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
      isActive={match.params.exampleIdCan === exampleIdCan}
    >
      <div />

      <Text className='flex-1 truncate'>{example.name}</Text>

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
    </TreeItem>
  );
};
