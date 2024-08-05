import { useMutation, useQuery } from '@connectrpc/connect-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { flow, Match, Struct } from 'effect';

import { ItemApiCall } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

export const CollectionListPage = () => {
  const collectionsQuery = useQuery(CollectionQuery.listCollections);
  if (!collectionsQuery.isSuccess) return null;
  const collections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Collections</h2>
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
