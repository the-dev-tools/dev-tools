import { createConnectQueryKey, useMutation, useQuery } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { flow, Match, Struct } from 'effect';
import { Button, FileTrigger } from 'react-aria-components';

import { ItemApiCall } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

export const CollectionListPage = () => {
  const collectionsQuery = useQuery(CollectionQuery.listCollections);
  if (!collectionsQuery.isSuccess) return null;
  const collections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Collections</h2>
      <ImportPostman />
      <div>
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
  const collectionQuery = useQuery(CollectionQuery.getCollection, { id });
  if (!collectionQuery.isSuccess) return null;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Collection</h2>
      <div>ID: {collectionQuery.data.id}</div>
      <div>Name: {collectionQuery.data.name}</div>
      <div>Nodes:</div>
      {collectionQuery.data.item.map(
        flow(
          Struct.get('itemData'),
          Match.value,
          Match.when({ case: 'itemApiCall' }, (_) => <ApiCall key={_.value.id} item={_.value} />),
          Match.orElse(() => null),
        ),
      )}
    </>
  );
};

interface ApiCallProps {
  item: ItemApiCall;
}

const ApiCall = ({ item }: ApiCallProps) => {
  const runNodeMutation = useMutation(CollectionQuery.runApiCall);

  return (
    <div>
      <span>{item.name} | </span>
      <button onClick={() => void runNodeMutation.mutate({ id: item.id })}>Run</button>
      {runNodeMutation.isSuccess && <span> | Status: {runNodeMutation.data.status}</span>}
    </div>
  );
};
