import {
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
  Text,
  TextArea,
  TextField,
  UNSTABLE_Tree,
  UNSTABLE_TreeItem,
  UNSTABLE_TreeItemContent,
} from 'react-aria-components';

import { ApiCall, CollectionMeta, Folder, Item } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

import { Runtime } from './runtime';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const CollectionsWidget = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createCollectionMutation = useMutation(CollectionQuery.createCollection);
  const collectionsQuery = useConnectQuery(CollectionQuery.listCollections, { workspaceId });

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

  if (!collectionsQuery.isSuccess) return null;
  const metaCollections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h3 className='uppercase'>Collections</h3>
      <div className='flex justify-between gap-2'>
        <Button
          onPress={async () => {
            await createCollectionMutation.mutateAsync({ workspaceId, name: 'New collection' });
            await queryClient.invalidateQueries(listQueryOptions);
          }}
          className='flex-1 rounded bg-black text-white'
        >
          New
        </Button>
        <ImportPostman />
      </div>
      {metaCollections.map((_) => (
        <Link
          key={_.id}
          to='/workspace/$workspaceId/collection/$collectionId'
          params={{ workspaceId, collectionId: _.id }}
        >
          {_.name}
        </Link>
      ))}
      <div>Tree</div>
      <UNSTABLE_Tree items={metaCollections}>{(meta) => <CollectionTreeItem meta={meta} />}</UNSTABLE_Tree>
    </>
  );
};

const ImportPostman = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createMutation = useMutation(CollectionQuery.importPostman);

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

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
      <Button className='flex-1 rounded bg-black text-white'>Import</Button>
    </FileTrigger>
  );
};

interface CollectionTreeItemProps {
  meta: CollectionMeta;
}

const CollectionTreeItem = ({ meta }: CollectionTreeItemProps) => {
  return (
    <UNSTABLE_TreeItem textValue={meta.name}>
      <UNSTABLE_TreeItemContent>
        <Text>{meta.name}</Text>
      </UNSTABLE_TreeItemContent>
    </UNSTABLE_TreeItem>
  );
};

const collectionRoute = getRouteApi('/_authorized/workspace/$workspaceId/collection/$collectionId');

class CollectionForm extends Schema.Class<CollectionForm>('CollectionForm')({
  name: Schema.String,
}) {}

export const CollectionPage = () => {
  const { workspaceId, collectionId } = collectionRoute.useParams();

  const router = useRouter();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteCollection);
  const updateMutation = useMutation(CollectionQuery.updateCollection);
  const createFolderMutation = useMutation(CollectionQuery.createFolder);

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  if (!query.isSuccess) return null;
  const { data } = query;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>{data.name}</h2>

      <Form
        key={data.id}
        className='my-2 border-y border-black py-2'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(CollectionForm),
            );

            yield* Effect.tryPromise(() => updateMutation.mutateAsync({ id: collectionId, name }));

            queryClient.setQueriesData(
              queryOptions,
              createProtobufSafeUpdater(
                CollectionQuery.getCollection,
                Struct.evolve({
                  name: () => name,
                }),
              ),
            );

            yield* Effect.tryPromise(() => queryClient.invalidateQueries(listQueryOptions));
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
              await deleteMutation.mutateAsync({ id: collectionId });
              await router.navigate({ to: '/workspace/$workspaceId', params: { workspaceId } });
              await queryClient.invalidateQueries(queryOptions);
              await queryClient.invalidateQueries(listQueryOptions);
            }}
          >
            Delete
          </Button>

          <Button
            onPress={async () => {
              await createFolderMutation.mutateAsync({ collectionId, name: 'New folder' });
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

class FolderForm extends Schema.Class<FolderForm>('FolderForm')({
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

  const { collectionId } = collectionRoute.useParams();
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
              Schema.decode(FolderForm),
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

  const { workspaceId, collectionId } = collectionRoute.useParams();
  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });

  return (
    <div className='flex gap-2'>
      <div>{apiCall.data!.method}</div>
      <div className='flex-1 truncate'>{apiCall.meta!.name}</div>
      {runNodeMutation.isSuccess && <div>Duration: {runNodeMutation.data.result!.duration.toString()} ms</div>}
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ collectionId, id: apiCall.meta!.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
      <Link to='/workspace/$workspaceId/api-call/$apiCallId' params={{ workspaceId, apiCallId: apiCall.meta!.id }}>
        Edit
      </Link>
      <Button onPress={() => void runNodeMutation.mutate({ id: apiCall.meta!.id })}>Run</Button>
    </div>
  );
};

const apiCallRoute = getRouteApi('/_authorized/workspace/$workspaceId/api-call/$apiCallId');

class ApiCallForm extends Schema.Class<ApiCallForm>('ApiCallForm')({
  name: Schema.String,
  method: Schema.Literal('GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'),
  url: Schema.String,
  body: Schema.String,
}) {}

export const ApiCallPage = () => {
  const { apiCallId } = apiCallRoute.useParams();

  const updateMutation = useMutation(CollectionQuery.updateApiCall);

  const query = useConnectQuery(CollectionQuery.getApiCall, { id: apiCallId });
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
              Schema.decode(ApiCallForm),
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
              {Array.map(ApiCallForm.fields.method.literals, (_) => (
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
