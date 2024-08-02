import { useMutation, useQuery } from '@connectrpc/connect-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import { CollectionNode } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
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
      <div>Nodes:</div>
      {collectionQuery.data.nodes.map((_) => (
        <Node key={_.id} node={_} />
      ))}
    </>
  );
};

interface NodeProps {
  node: CollectionNode;
}

const Node = ({ node }: NodeProps) => {
  const runNodeMutation = useMutation(CollectionQuery.runNode);

  return (
    <div>
      <span>{node.name} | </span>
      <button onClick={() => void runNodeMutation.mutate({ id: node.id })}>Run</button>
      {runNodeMutation.isSuccess && <span> | Status: {runNodeMutation.data.status}</span>}
    </div>
  );
};
