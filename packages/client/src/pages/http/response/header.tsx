import { eq, useLiveQuery } from '@tanstack/react-db';
import { HttpResponseHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';

export interface HeaderTableProps {
  httpResponseId: Uint8Array;
}

export const HeaderTable = ({ httpResponseId }: HeaderTableProps) => {
  const collection = useApiCollection(HttpResponseHeaderCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpResponseId, httpResponseId))
        .select((_) => pick(_.item, 'key', 'value')),
    [collection, httpResponseId],
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
