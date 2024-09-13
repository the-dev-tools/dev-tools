import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link, Outlet, useMatch } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, Effect, Match, pipe, Struct } from 'effect';
import { useEffect, useMemo, useRef, useState } from 'react';
import { FileTrigger, Form, MenuTrigger, Text } from 'react-aria-components';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuFolder, LuImport, LuMoreHorizontal, LuPlus, LuSave, LuSendHorizonal } from 'react-icons/lu';
import { useDebouncedCallback } from 'use-debounce';

import { CollectionMeta } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import {
  createCollection,
  deleteCollection,
  importPostman,
  listCollections,
  updateCollection,
} from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { ApiCallMeta, GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import {
  deleteApiCall,
  getApiCall,
  updateApiCall,
} from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Header } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createHeader,
  updateHeader,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { FolderMeta, ItemMeta } from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder_pb';
import {
  createFolder,
  deleteFolder,
  updateFolder,
} from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder-ItemFolderService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Popover } from '@the-dev-tools/ui/popover';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { Tree, TreeItem } from '@the-dev-tools/ui/tree';

import { Runtime } from './runtime';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const CollectionsTree = () => {
  const { workspaceId } = workspaceRoute.useParams();

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
  const { workspaceId } = workspaceRoute.useParams();

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
  const { workspaceId } = workspaceRoute.useParams();

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

  const { workspaceId } = workspaceRoute.useParams();

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
  const { workspaceId } = workspaceRoute.useParams();

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

const apiCallRoute = getRouteApi('/_authorized/workspace/$workspaceId/api-call/$apiCallId');

export const ApiCallPage = () => {
  const { apiCallId } = apiCallRoute.useParams();

  const query = useConnectQuery(getApiCall, { id: apiCallId });

  if (!query.isSuccess) return null;
  const { data } = query;

  return <ApiCallForm data={data} />;
};

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

class ApiCallFormData extends Schema.Class<ApiCallFormData>('ApiCallFormData')({
  method: Schema.String.pipe(Schema.filter((_) => Array.contains(methods, _) || 'Method is not valid')),
  url: Schema.String.pipe(Schema.nonEmptyString({ message: () => 'URL must not be empty' })),
}) {}

interface ApiCallFormProps {
  data: GetApiCallResponse;
}

const ApiCallForm = ({ data }: ApiCallFormProps) => {
  const { workspaceId, apiCallId } = apiCallRoute.useParams();

  const updateMutation = useConnectMutation(updateApiCall);

  const form = useForm({
    resolver: effectTsResolver(ApiCallFormData),
    defaultValues: data.apiCall!,
  });

  return (
    <div className='flex h-full flex-col'>
      <Form
        onSubmit={form.handleSubmit((formData) => {
          const newApiCall = Struct.evolve(data.apiCall!, {
            method: () => formData.method,
            url: () => formData.url,
          });

          updateMutation.mutate({ apiCall: newApiCall });
        })}
      >
        <div className='flex items-center gap-2 border-b-2 border-black px-4 py-3'>
          <h2 className='flex-1 truncate text-sm font-bold'>{data.apiCall!.meta!.name}</h2>

          <Button kind='placeholder' variant='placeholder' type='submit'>
            <LuSave /> Save
          </Button>
        </div>

        <div className='flex items-start p-4 pb-0'>
          <SelectRHF
            control={form.control}
            name='method'
            aria-label='Method'
            triggerClassName={tw`rounded-r-none border-r-0`}
          >
            {methods.map((_) => (
              <DropdownItem key={_} id={_}>
                {_}
              </DropdownItem>
            ))}
          </SelectRHF>

          <TextFieldRHF
            control={form.control}
            name='url'
            aria-label='URL'
            className={tw`flex-1`}
            inputClassName={tw`rounded-none border-x-0 bg-neutral-200`}
          />

          {/* TODO: implement */}
          <Button kind='placeholder' variant='placeholder' className='rounded-l-none border-l-0 bg-black text-white'>
            Send <LuSendHorizonal className='size-4' />
          </Button>
        </div>
      </Form>

      <div className='flex flex-1 flex-col gap-4 p-4'>
        <div className='flex gap-4 border-b border-black'>
          <Link
            className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
            activeProps={{ className: tw`border-b-black` }}
            activeOptions={{ exact: true }}
            from='/workspace/$workspaceId/api-call/$apiCallId'
            to='.'
          >
            Params
          </Link>
          <Link
            className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
            activeProps={{ className: tw`border-b-black` }}
            from='/workspace/$workspaceId/api-call/$apiCallId'
            to='headers'
            params={{ workspaceId, apiCallId }}
          >
            Headers
          </Link>
        </div>

        <Outlet />
      </div>
    </div>
  );
};

export const ApiCallHeaderTab = () => {
  const { apiCallId } = apiCallRoute.useParams();
  const query = useConnectQuery(getApiCall, { id: apiCallId });
  if (!query.isSuccess) return null;
  return <ApiCallHeaderForm data={query.data} />;
};

interface ApiCallHeaderFormProps {
  data: GetApiCallResponse;
}

const ApiCallHeaderForm = ({ data }: ApiCallHeaderFormProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const getQueryOptions = createQueryOptions(getApiCall, { id: data.apiCall!.meta!.id }, { transport });

  const values = useMemo(
    () => ({
      header: [...data.example!.header, new Header({ enabled: true })],
    }),
    [data.example],
  );

  const form = useForm({ values });
  const fieldArray = useFieldArray({ name: 'header', control: form.control });

  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<Header>();
    return [
      accessor('enabled', {
        header: '',
        minSize: 0,
        size: 0,
        cell: ({ row }) => <CheckboxRHF control={form.control} name={`header.${row.index}.enabled`} className='p-1' />,
      }),
      accessor('key', {
        cell: ({ row }) => (
          <TextFieldRHF
            control={form.control}
            name={`header.${row.index}.key`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('value', {
        cell: ({ row }) => (
          <TextFieldRHF
            control={form.control}
            name={`header.${row.index}.value`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF
            control={form.control}
            name={`header.${row.index}.description`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
    ];
  }, [form.control]);

  const table = useReactTable({
    columns,
    data: fieldArray.fields,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.id,
  });

  const updateHeaderMutation = useConnectMutation(updateHeader);
  const createHeaderMutation = useConnectMutation(createHeader);

  const updateHeaderMap = useRef(new Map<string, Header>());
  const updateHeaders = useDebouncedCallback(async () => {
    const headers = updateHeaderMap.current;

    const promises = Array.fromIterable(headers.values()).map(async (header) => {
      if (header.id) return void (await updateHeaderMutation.mutateAsync({ header }));

      await createHeaderMutation.mutateAsync({ header: { ...header, exampleId: data.example!.meta!.id } });
      await queryClient.invalidateQueries(getQueryOptions);
    });

    headers.clear();
    await Promise.allSettled(promises);
  }, 200);

  useEffect(() => {
    const watch = form.watch((_, { name }) => {
      const rowName = name?.match(/(^header.[\d]+)/g)?.[0] as `header.${number}` | undefined;
      if (!rowName) return;
      const rowValues = form.getValues(rowName);
      updateHeaderMap.current.set(rowValues.id, rowValues);
      void updateHeaders();
    });
    return () => void watch.unsubscribe();
  }, [form, updateHeaders]);

  useEffect(() => () => void updateHeaders.flush(), [updateHeaders]);

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
                <td key={cell.id} className='break-all align-middle text-sm'>
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
