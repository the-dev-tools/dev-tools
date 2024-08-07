import {
  createConnectQueryKey,
  createQueryOptions,
  useQuery as useConnectQuery,
  useMutation,
  useTransport,
} from '@connectrpc/connect-query';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link, useRouter } from '@tanstack/react-router';
import { Boolean, Match, pipe, Struct } from 'effect';
import { useState } from 'react';
import { Button, FileTrigger } from 'react-aria-components';

import { ApiCall, Folder, Item } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

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

export const CollectionEditPage = () => {
  const { id } = collectionEditRoute.useParams();

  const router = useRouter();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteCollection);

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  if (!query.isSuccess) return null;
  const { data } = query;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>{data.name}</h2>
      <div>
        <button
          onClick={async () => {
            await deleteMutation.mutateAsync({ id });
            await router.navigate({ to: '/collections' });
            await queryClient.invalidateQueries(queryOptions);
          }}
        >
          Delete collection
        </button>
      </div>
      {data.items.map((_) => (
        <ItemRow key={_.data.value?.meta?.id ?? ''} item={_} />
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

interface FolderRowProps {
  folder: Folder;
}

const FolderRow = ({ folder }: FolderRowProps) => {
  const [open, setOpen] = useState(false);

  const row = (
    <div className='flex gap-2'>
      <div>FOLDER</div>
      <div className='flex-1'>{folder.meta?.name}</div>
      <button onClick={() => void setOpen(Boolean.not)}>{open ? 'Close' : 'Open'}</button>
    </div>
  );

  if (!open) return row;

  return (
    <>
      {row}
      <div className='border-l-2 border-black pl-2'>
        {folder.items.map((_) => (
          <ItemRow key={_.data.value?.meta?.id ?? ''} item={_} />
        ))}
      </div>
    </>
  );
};

interface ApiCallRowProps {
  apiCall: ApiCall;
}

const ApiCallRow = ({ apiCall }: ApiCallRowProps) => {
  const runNodeMutation = useMutation(CollectionQuery.runApiCall);

  return (
    <div className='flex gap-2'>
      <div>{apiCall.data?.method}</div>
      <div className='flex-1 truncate'>{apiCall.meta?.name}</div>
      {runNodeMutation.isSuccess && <div>Status: {runNodeMutation.data.status}</div>}
      <button onClick={() => void runNodeMutation.mutate({ id: apiCall.meta?.id ?? '' })}>Run</button>
    </div>
  );
};
