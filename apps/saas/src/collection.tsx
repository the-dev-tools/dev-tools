import { useQuery } from '@connectrpc/connect-query';

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
          <div key={_.id}>{_.name}</div>
        ))}
      </div>
    </>
  );
};
