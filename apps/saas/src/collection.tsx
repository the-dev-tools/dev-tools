import { useQuery } from '@connectrpc/connect-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

export const CollectionListPage = () => {
  const collectionsQuery = useQuery(CollectionQuery.listCollections);
  if (!collectionsQuery.isSuccess) return null;
  const collections = collectionsQuery.data.simpleCollections;

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
  const collectionQuery = useQuery(CollectionQuery.getCollectionWithNode, { id });
  if (!collectionQuery.isSuccess) return null;

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Collection</h2>
      <div>ID: {collectionQuery.data.id}</div>
      <div>Name: {collectionQuery.data.name}</div>
      <div>Nodes: {JSON.stringify(collectionQuery.data.nodes)}</div>
    </>
  );
};
