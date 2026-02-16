import { eq, useLiveQuery } from '@tanstack/react-db';
import { Fragment } from 'react/jsx-runtime';
import { twJoin } from 'tailwind-merge';
import { GraphQLResponseAssertCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';

export interface GraphQLAssertTableProps {
  graphqlResponseId: Uint8Array;
}

export const GraphQLAssertTable = ({ graphqlResponseId }: GraphQLAssertTableProps) => {
  const collection = useApiCollection(GraphQLResponseAssertCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.graphqlResponseId, graphqlResponseId))
        .select((_) => pick(_.item, 'graphqlResponseAssertId', 'value', 'success')),
    [collection, graphqlResponseId],
  );

  return (
    <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
      {items.map((_) => (
        <Fragment key={collection.utils.getKey(_)}>
          <div
            className={twJoin(
              tw`rounded-sm px-2 py-1 text-center font-light text-on-inverse uppercase`,
              _.success ? tw`bg-success` : tw`bg-danger`,
            )}
          >
            {_.success ? 'Pass' : 'Fail'}
          </div>

          <span>{_.value}</span>
        </Fragment>
      ))}
    </div>
  );
};
