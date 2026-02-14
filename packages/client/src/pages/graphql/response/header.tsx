import { eq, useLiveQuery } from '@tanstack/react-db';
import { GraphQLResponseHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from '@the-dev-tools/ui/table';
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

  return (
    <Table aria-label='Response headers'>
      <TableHeader>
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
      </TableHeader>

      <TableBody items={items}>
        {(_) => (
          <TableRow id={_.key}>
            <TableCell className={tw`px-5 py-1.5`}>{_.key}</TableCell>
            <TableCell className={tw`px-5 py-1.5`}>{_.value}</TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
};
