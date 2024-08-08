import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useQuery as useConnectQuery,
  useMutation,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link, useRouter } from '@tanstack/react-router';
import { Array, Boolean, Effect, Match, pipe, Struct } from 'effect';
import { useState } from 'react';
import {
  Button,
  FileTrigger,
  Form,
  Input,
  Label,
  ListBox,
  ListBoxItem,
  Popover,
  Select,
  SelectValue,
  TextArea,
  TextField,
} from 'react-aria-components';

import { ApiCall, Folder, Item } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

import { Runtime } from './runtime';

export const CollectionListPage = () => {
  const router = useRouter();
  const createCollectionMutation = useMutation(CollectionQuery.createCollection);
  const collectionsQuery = useConnectQuery(CollectionQuery.listCollections);

  if (!collectionsQuery.isSuccess) return null;
  const collections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Collections</h2>
      <div className='flex justify-between'>
        <ImportPostman />
        <button
          onClick={async () => {
            const response = await createCollectionMutation.mutateAsync({ name: 'New collection' });
            await router.navigate({ to: '/collection/$id', params: { id: response.id } });
          }}
        >
          Create collection
        </button>
      </div>
      <div className='mt-4 flex flex-col'>
        {collections.map((_) => (
          <Link key={_.id} to='/collection/$id' params={{ id: _.id }}>
            {_.name}
          </Link>
        ))}
      </div>
    </>
  );
};

const ImportPostman = () => {
  const queryClient = useQueryClient();
  const createMutation = useMutation(CollectionQuery.importPostman);

  return (
    <div>
      <span>Import Postman collection: </span>
      <FileTrigger
        onSelect={async (_) => {
          const file = _?.item(0);
          if (!file) return;
          await createMutation.mutateAsync({
            name: file.name,
            data: new Uint8Array(await file.arrayBuffer()),
          });
          await queryClient.invalidateQueries({
            queryKey: createConnectQueryKey(CollectionQuery.listCollections),
          });
        }}
      >
        <Button>Select a file</Button>
      </FileTrigger>
    </div>
  );
};

const collectionEditRoute = getRouteApi('/authenticated/dashboard/collection/$id');

class CollectionUpdateForm extends Schema.Class<CollectionUpdateForm>('CollectionUpdateForm')({
  name: Schema.String,
}) {}

export const CollectionEditPage = () => {
  const { id } = collectionEditRoute.useParams();

  const router = useRouter();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteCollection);
  const updateMutation = useMutation(CollectionQuery.updateCollection);
  const createFolderMutation = useMutation(CollectionQuery.createFolder);

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  if (!query.isSuccess) return null;
  const { data } = query;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>{data.name}</h2>

      <Form
        className='my-2 border-y border-black py-2'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(CollectionUpdateForm),
            );

            yield* Effect.tryPromise(() => updateMutation.mutateAsync({ id, name }));

            queryClient.setQueriesData(
              queryOptions,
              createProtobufSafeUpdater(
                CollectionQuery.getCollection,
                Struct.evolve({
                  name: () => name,
                }),
              ),
            );
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField defaultValue={data.name} name='name' className='flex gap-2'>
          <Label>Name:</Label>
          <Input />
        </TextField>

        <div className='flex gap-2'>
          <Button type='submit'>Save</Button>

          <Button
            onPress={async () => {
              await deleteMutation.mutateAsync({ id });
              await router.navigate({ to: '/collections' });
              await queryClient.invalidateQueries(queryOptions);
            }}
          >
            Delete
          </Button>

          <Button
            onPress={async () => {
              await createFolderMutation.mutateAsync({ collectionId: id, name: 'New folder' });
              await queryClient.invalidateQueries(queryOptions);
            }}
          >
            Create folder
          </Button>
        </div>
      </Form>

      {data.items.map((_) => (
        <ItemRow key={_.data.value!.meta!.id} item={_} />
      ))}
    </>
  );
};

interface ItemRowProps {
  item: Item;
}

const ItemRow = ({ item }: ItemRowProps) =>
  pipe(
    item,
    Struct.get('data'),
    Match.value,
    Match.when({ case: 'apiCall' }, (_) => <ApiCallRow apiCall={_.value} />),
    Match.when({ case: 'folder' }, (_) => <FolderRow folder={_.value} />),
    Match.orElse(() => null),
  );

class FolderUpdateForm extends Schema.Class<FolderUpdateForm>('FolderUpdateForm')({
  name: Schema.String,
}) {}

interface FolderRowProps {
  folder: Folder;
}

const FolderRow = ({ folder }: FolderRowProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteFolder);
  const updateMutation = useMutation(CollectionQuery.updateFolder);

  const { id: collectionId } = collectionEditRoute.useParams();
  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });

  const [open, setOpen] = useState(false);

  const row = (
    <div className='flex gap-2'>
      <div>FOLDER</div>
      <Form
        className='flex-1'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(FolderUpdateForm),
            );

            yield* Effect.tryPromise(() =>
              updateMutation.mutateAsync({
                folder: pipe(
                  folder,
                  Struct.omit('items'),
                  Struct.evolve({
                    meta: Struct.evolve({
                      name: () => name,
                    }),
                  }),
                ),
              }),
            );

            yield* Effect.tryPromise(() => queryClient.invalidateQueries(queryOptions));
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField name='name' aria-label='Folder name' defaultValue={folder.meta!.name}>
          <Input className='w-full bg-transparent' />
        </TextField>
      </Form>
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ collectionId, id: folder.meta!.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
      <Button onPress={() => void setOpen(Boolean.not)}>{open ? 'Close' : 'Open'}</Button>
    </div>
  );

  if (!open) return row;

  return (
    <>
      {row}
      <div className='border-l-2 border-black pl-2'>
        {folder.items.map((_) => (
          <ItemRow key={_.data.value!.meta!.id} item={_} />
        ))}
      </div>
    </>
  );
};

interface ApiCallRowProps {
  apiCall: ApiCall;
}

const ApiCallRow = ({ apiCall }: ApiCallRowProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const runNodeMutation = useMutation(CollectionQuery.runApiCall);
  const deleteMutation = useMutation(CollectionQuery.deleteApiCall);

  const { id: collectionId } = collectionEditRoute.useParams();
  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });

  return (
    <div className='flex gap-2'>
      <div>{apiCall.data!.method}</div>
      <div className='flex-1 truncate'>{apiCall.meta!.name}</div>
      {runNodeMutation.isSuccess && <div>Status: {runNodeMutation.data.status}</div>}
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ collectionId, id: apiCall.meta!.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
      <Link to='/api-call/$id' params={{ id: apiCall.meta!.id }}>
        Edit
      </Link>
      <Button onPress={() => void runNodeMutation.mutate({ id: apiCall.meta!.id })}>Run</Button>
    </div>
  );
};

const apiCallEditRoute = getRouteApi('/authenticated/dashboard/api-call/$id');

class ApiCallEditForm extends Schema.Class<ApiCallEditForm>('ApiCallEditForm')({
  name: Schema.String,
  method: Schema.Literal('GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'),
  url: Schema.String,
  body: Schema.String,
}) {}

export const ApiCallEditPage = () => {
  const { id } = apiCallEditRoute.useParams();

  const updateMutation = useMutation(CollectionQuery.updateApiCall);

  const query = useConnectQuery(CollectionQuery.getApiCall, { id });
  if (!query.isSuccess) return null;
  const { data } = query;

  const body = new TextDecoder().decode(data.apiCall!.data!.body);

  return (
    <>
      <h2 className='truncate text-center text-2xl font-extrabold'>{data.apiCall!.meta!.name}</h2>

      <div className='my-2 h-px bg-black' />

      <Form
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name, method, url, body } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(ApiCallEditForm),
            );

            const newApiCall = Struct.evolve(data.apiCall!, {
              meta: (_) => Struct.evolve(_!, { name: () => name }),
              data: (_) =>
                Struct.evolve(_!, {
                  method: () => method,
                  url: () => url,
                  body: () => new TextEncoder().encode(body),
                }),
            });

            yield* Effect.tryPromise(() => updateMutation.mutateAsync({ apiCall: newApiCall }));
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField name='name' defaultValue={data.apiCall!.meta!.name} className='flex gap-2'>
          <Label>Name:</Label>
          <Input className='flex-1' />
        </TextField>

        <Select name='method' defaultSelectedKey={data.apiCall!.data!.method} className='flex gap-2'>
          <Label>Method:</Label>
          <Button>
            <SelectValue />
          </Button>
          <Popover className='bg-white'>
            <ListBox>
              {Array.map(ApiCallEditForm.fields.method.literals, (_) => (
                <ListBoxItem key={_} id={_} className='cursor-pointer'>
                  {_}
                </ListBoxItem>
              ))}
            </ListBox>
          </Popover>
        </Select>

        <TextField name='url' defaultValue={data.apiCall!.data!.url} className='flex gap-2'>
          <Label>URL:</Label>
          <Input className='flex-1' />
        </TextField>

        <TextField name='body' defaultValue={body} className='flex gap-2'>
          <Label>Body:</Label>
          <TextArea className='flex-1' />
        </TextField>

        <Button type='submit'>Save</Button>
      </Form>
    </>
  );
};
