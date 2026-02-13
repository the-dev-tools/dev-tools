import { eq, useLiveQuery } from '@tanstack/react-db';
import { createColumnHelper } from '@tanstack/react-table';
import { GraphQLResponseHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';

export interface GraphQLResponseHeaderTableProps {
  graphqlResponseId: Uint8Array;
}

export const GraphQLResponseHeaderTable = ({ graphqlResponseId }: GraphQLResponseHeaderTableProps) => {
  const collection = useApiCollection(GraphQLResponseHeaderCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.graphqlResponseId, graphqlResponseId))
        .select((_) => pick(_.item, 'key', 'value')),
    [collection, graphqlResponseId],
  );

  const { accessor } = createColumnHelper<(typeof items)[number]>();

  const columns = [
    accessor('key', { cell: ({ cell }) => <div className={tw`px-5 py-1.5`}>{cell.renderValue()}</div> }),
    accessor('value', { cell: ({ cell }) => <div className={tw`px-5 py-1.5`}>{cell.renderValue()}</div> }),
  ];

  const table = useReactTable({
    columns,
    data: items,
  });

  return <DataTable aria-label='Response headers' table={table} />;
};
