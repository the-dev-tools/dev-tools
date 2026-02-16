import { eq, useLiveQuery } from '@tanstack/react-db';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';

export interface GraphQLUrlProps {
  graphqlId: Uint8Array;
  isReadOnly?: boolean;
}

export const GraphQLUrl = ({ graphqlId, isReadOnly = false }: GraphQLUrlProps) => {
  const collection = useApiCollection(GraphQLCollectionSchema);

  const { url } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.graphqlId, graphqlId))
          .select((_) => pick(_.item, 'url'))
          .findOne(),
      [collection, graphqlId],
    ).data ?? {};

  return (
    <div className={tw`flex flex-1 items-center gap-3 rounded-lg border border-neutral px-3 py-2 shadow-xs`}>
      <ReferenceField
        aria-label='GraphQL Endpoint URL'
        className={tw`min-w-0 flex-1 border-none font-medium tracking-tight`}
        kind='StringExpression'
        readOnly={isReadOnly}
        value={url ?? ''}
      />
    </div>
  );
};
