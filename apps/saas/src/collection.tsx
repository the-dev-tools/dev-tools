import {
  createProtobufSafeUpdater,
  createQueryOptions,
  useQuery as useConnectQuery,
  useMutation,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, useMatch } from '@tanstack/react-router';
import { Array, Effect, Match, pipe, Struct } from 'effect';
import { useRef, useState } from 'react';
import { FileTrigger, Form, MenuTrigger, Text } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { LuFolder, LuImport, LuMoreHorizontal, LuPlus, LuSave, LuSendHorizonal } from 'react-icons/lu';

import { CollectionMeta } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import {
  createCollection,
  deleteCollection,
  getCollection,
  importPostman,
  listCollections,
  updateCollection,
} from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { ApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import {
  deleteApiCall,
  getApiCall,
  updateApiCall,
} from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Folder, Item } from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder_pb';
import {
  createFolder,
  deleteFolder,
  updateFolder,
} from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder-ItemFolderService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Popover } from '@the-dev-tools/ui/popover';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { Tree, TreeItem } from '@the-dev-tools/ui/tree';

import { Runtime } from './runtime';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const CollectionsWidget = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createCollectionMutation = useMutation(createCollection);
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
        {(_) => <CollectionWidget id={_.id} meta={_} />}
      </Tree>
    </>
  );
};

interface CollectionWidgetProps {
  id: string;
  meta: CollectionMeta;
}

const CollectionWidget = ({ meta }: CollectionWidgetProps) => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(deleteCollection);
  const updateMutation = useMutation(updateCollection);
  const createFolderMutation = useMutation(createFolder);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  const queryOptions = createQueryOptions(getCollection, { id: meta.id }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={query.data?.items ?? []}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={meta.id} />}
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
              await queryClient.invalidateQueries(queryOptions);
            }}
          >
            Delete
          </MenuItem>

          <MenuItem
            onAction={async () => {
              await createFolderMutation.mutateAsync({ collectionId: meta.id, name: 'New folder' });
              await queryClient.invalidateQueries(queryOptions);
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

              queryClient.setQueriesData(
                queryOptions,
                createProtobufSafeUpdater(
                  getCollection,
                  Struct.evolve({
                    name: () => name,
                  }),
                ),
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

interface ItemWidgetProps {
  id: string;
  item: Item;
  collectionId: string;
}

const ItemWidget = ({ item, collectionId }: ItemWidgetProps) =>
  pipe(
    item,
    Struct.get('data'),
    Match.value,
    Match.when({ case: 'folder' }, (_) => <FolderWidget folder={_.value} collectionId={collectionId} />),
    Match.when({ case: 'apiCall' }, (_) => <ApiCallWidget apiCall={_.value} collectionId={collectionId} />),
    Match.orElse(() => null),
  );

interface FolderWidgetProps {
  folder: Folder;
  collectionId: string;
}

const FolderWidget = ({ folder, collectionId }: FolderWidgetProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(deleteFolder);
  const updateMutation = useMutation(updateFolder);

  const queryOptions = createQueryOptions(getCollection, { id: collectionId }, { transport });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={folder.meta!.name}
      childItems={folder.items}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={collectionId} />}
    >
      <LuFolder />

      <Text ref={triggerRef} className='flex-1 truncate'>
        {folder.meta!.name}
      </Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem
            onAction={async () => {
              await deleteMutation.mutateAsync({ collectionId, id: folder.meta!.id });
              await queryClient.invalidateQueries(queryOptions);
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
                  folder: {
                    ...Struct.evolve(folder, {
                      meta: Struct.evolve({ name: () => name }),
                    }),
                    collectionId,
                  },
                }),
              );

              yield* Effect.tryPromise(() => queryClient.invalidateQueries(queryOptions));

              setIsRenaming(false);
            }).pipe(Runtime.runPromise)
          }
        >
          <TextField
            name='name'
            defaultValue={folder.meta!.name}
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

interface ApiCallWidgetProps {
  apiCall: ApiCall;
  collectionId: string;
}

const ApiCallWidget = ({ apiCall, collectionId }: ApiCallWidgetProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const match = useMatch({ strict: false });

  const { workspaceId } = workspaceRoute.useParams();

  const deleteMutation = useMutation(deleteApiCall);

  const queryOptions = createQueryOptions(getCollection, { id: collectionId }, { transport });

  return (
    <TreeItem
      textValue={apiCall.meta!.name}
      href={{ to: '/workspace/$workspaceId/api-call/$apiCallId', params: { workspaceId, apiCallId: apiCall.meta!.id } }}
      wrapperIsSelected={match.params.apiCallId === apiCall.meta!.id}
    >
      <div />

      <div className='text-sm font-bold'>{apiCall.method}</div>

      <Text className='flex-1 truncate'>{apiCall.meta!.name}</Text>

      <MenuTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
          <LuMoreHorizontal />
        </Button>

        <Menu>
          <MenuItem
            onAction={async () => {
              await deleteMutation.mutateAsync({ collectionId, id: apiCall.meta!.id });
              await queryClient.invalidateQueries(queryOptions);
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

  const createMutation = useMutation(importPostman);

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

  return <ApiCallForm data={data.apiCall!} />;
};

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

class ApiCallFormData extends Schema.Class<ApiCallFormData>('ApiCallFormData')({
  method: Schema.String.pipe(Schema.filter((_) => Array.contains(methods, _) || 'Method is not valid')),
  url: Schema.String.pipe(Schema.nonEmptyString({ message: () => 'URL must not be empty' })),
}) {}

interface ApiCallFormProps {
  data: ApiCall;
}

const ApiCallForm = ({ data }: ApiCallFormProps) => {
  const updateMutation = useMutation(updateApiCall);

  const form = useForm({
    resolver: effectTsResolver(ApiCallFormData),
    defaultValues: data,
  });

  return (
    <Form
      onSubmit={form.handleSubmit((formData) => {
        const newApiCall = Struct.evolve(data, {
          method: () => formData.method,
          url: () => formData.url,
        });

        updateMutation.mutate({ apiCall: newApiCall });
      })}
    >
      <div className='flex items-center gap-2 border-b-2 border-black px-4 py-3'>
        <h2 className='flex-1 truncate text-sm font-bold'>{data.meta!.name}</h2>

        <Button kind='placeholder' variant='placeholder' type='submit'>
          <LuSave /> Save
        </Button>
      </div>

      <div className='flex items-start p-4'>
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
  );
};
