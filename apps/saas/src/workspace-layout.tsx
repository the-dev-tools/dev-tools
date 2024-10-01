import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Outlet, redirect, useMatch } from '@tanstack/react-router';
import { flexRender } from '@tanstack/react-table';
import { Effect, Match, pipe, Struct } from 'effect';
import { useCallback, useRef, useState } from 'react';
import {
  Dialog,
  DialogTrigger,
  FileTrigger,
  Form,
  MenuTrigger,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  Text,
} from 'react-aria-components';
import { LuBraces, LuClipboardList, LuFolder, LuImport, LuMoreHorizontal, LuPlus, LuX } from 'react-icons/lu';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';

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
import { ApiExampleMeta } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import { deleteExample } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { FolderMeta, ItemMeta } from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder_pb';
import {
  createFolder,
  deleteFolder,
  updateFolder,
} from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder-ItemFolderService_connectquery';
import { getWorkspace } from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { Popover } from '@the-dev-tools/ui/popover';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import { Tree, TreeItem } from '@the-dev-tools/ui/tree';

import { DashboardLayout } from './authorized';
import { useFormTable } from './form-table';
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
        <Panel className='flex flex-col' style={{ overflowY: 'auto' }} defaultSize={20} minSize={10} maxSize={40}>
          <div className='flex justify-between border-b border-black p-2'>
            {/* TODO: connect with backend once implemented */}
            <Select
              aria-label='Environment'
              selectedKey='development'
              triggerClassName={tw`justify-start`}
              triggerVariant='placeholder ghost'
            >
              <DropdownItem id='development' textValue='development'>
                <div className='flex items-center gap-2 text-sm'>
                  <div className='flex size-7 items-center justify-center rounded-md border border-black bg-neutral-200'>
                    D
                  </div>
                  <span className='font-semibold'>Development</span>
                </div>
              </DropdownItem>
              <DropdownItem id='test' textValue='test'>
                <div className='flex items-center gap-2 text-sm'>
                  <div className='flex size-7 items-center justify-center rounded-md border border-black bg-neutral-200'>
                    D
                  </div>
                  <span className='font-semibold'>Testing</span>
                </div>
              </DropdownItem>
              <DropdownItem id='production' textValue='production'>
                <div className='flex items-center gap-2 text-sm'>
                  <div className='flex size-7 items-center justify-center rounded-md border border-black bg-neutral-200'>
                    D
                  </div>
                  <span className='font-semibold'>Production</span>
                </div>
              </DropdownItem>
            </Select>

            <DialogTrigger>
              <Button kind='placeholder' variant='placeholder ghost' className='aspect-square'>
                <LuClipboardList />
              </Button>

              <Modal modalClassName={tw`size-full`}>
                <Dialog className='h-full outline-none'>
                  {({ close }) => (
                    <Tabs className='flex h-full'>
                      <div className='flex w-72 flex-col gap-2 border-r border-black bg-neutral-200 p-4'>
                        <div className='-order-3 mb-2'>
                          <div className='text-lg font-medium'>Variable Settings</div>
                          <span className='text-sm font-light'>Manage variables & environment</span>
                        </div>

                        <div className='-order-1 text-neutral-600'>Environments</div>

                        <TabList className='contents'>
                          <Tab
                            id='global'
                            className={({ isSelected }) =>
                              twJoin(
                                tw`-order-2 -m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                                isSelected && tw`bg-neutral-400`,
                              )
                            }
                          >
                            <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>
                              <LuBraces />
                            </div>
                            <span>Global Variables</span>
                          </Tab>
                          <Tab
                            id='development'
                            className={({ isSelected }) =>
                              twJoin(
                                tw`-m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                                isSelected && tw`bg-neutral-400`,
                              )
                            }
                          >
                            <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>D</div>
                            <span>Development</span>
                          </Tab>
                          <Tab
                            id='testing'
                            className={({ isSelected }) =>
                              twJoin(
                                tw`-m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                                isSelected && tw`bg-neutral-400`,
                              )
                            }
                          >
                            <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>T</div>
                            <span>Testing</span>
                          </Tab>
                          <Tab
                            id='production'
                            className={({ isSelected }) =>
                              twJoin(
                                tw`-m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                                isSelected && tw`bg-neutral-400`,
                              )
                            }
                          >
                            <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>P</div>
                            <span>Production</span>
                          </Tab>
                        </TabList>
                      </div>

                      <TabPanel id='global' className='flex h-full flex-1 flex-col'>
                        <div className='px-6 py-4'>
                          <div className='mb-4 flex items-start'>
                            <div className='flex-1'>
                              <h1 className='text-xl font-medium'>Global Variables</h1>
                              <span className='text-sm font-light'>
                                Lorem ipsum dolor sit amet consectur adipiscing elit.
                              </span>
                            </div>

                            <Button variant='placeholder ghost' kind='placeholder' onPress={close}>
                              <LuX />
                            </Button>
                          </div>

                          <EnvironmentVariables />
                        </div>

                        <div className='flex-1' />

                        <div className='flex justify-end border-t border-black bg-neutral-100 px-6 py-4'>
                          <Button kind='placeholder' variant='placeholder' onPress={close}>
                            Save
                          </Button>
                        </div>
                      </TabPanel>
                    </Tabs>
                  )}
                </Dialog>
              </Modal>
            </DialogTrigger>
          </div>

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

interface EnvironmentVariable {
  id: string;
  enabled: boolean;
  key: string;
  value: string;
  description: string;
}

// TODO: connect to BE when implemented
const EnvironmentVariables = () => {
  const dummyCallback = useCallback(() => Promise.resolve({ id: Math.random().toString() }), []);

  const table = useFormTable<EnvironmentVariable>({
    items: [],
    makeItem: (item) => ({ id: '', enabled: true, key: '', value: '', description: '', ...item }),
    onCreate: dummyCallback,
    onUpdate: dummyCallback,
    onDelete: dummyCallback,
  });

  return (
    <div className='rounded border border-black'>
      <table className='w-full divide-inherit border-inherit'>
        <thead className='divide-y divide-inherit border-b border-inherit'>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className='p-1.5 text-left text-sm font-normal capitalize text-neutral-500'
                  style={{ width: ((header.getSize() / table.getTotalSize()) * 100).toString() + '%' }}
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className='divide-y divide-inherit'>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className='break-all p-1 align-middle text-sm'>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
      href={{
        to: '/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId',
        params: { workspaceId, apiCallId: meta.id, exampleId: meta.defaultExampleId },
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

interface ApiExampleItemProps {
  apiCallId: string;
  meta: ApiExampleMeta;
}

const ApiExampleItem = ({ apiCallId, meta }: ApiExampleItemProps) => {
  const match = useMatch({ strict: false });

  const { workspaceId } = Route.useParams();

  const deleteMutation = useConnectMutation(deleteExample);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  return (
    <TreeItem
      textValue={meta.name}
      href={{
        to: '/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId',
        params: { workspaceId, apiCallId: apiCallId, exampleId: meta.id },
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
