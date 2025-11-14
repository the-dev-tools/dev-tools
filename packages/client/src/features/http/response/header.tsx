import { eq, useLiveQuery } from '@tanstack/react-db';
import { createColumnHelper } from '@tanstack/react-table';
import { HttpResponseHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { pick } from '~/utils/tanstack-db';

export interface HeaderTableProps {
  httpId: Uint8Array;
}

export const HeaderTable = ({ httpId }: HeaderTableProps) => {
  const collection = useApiCollection(HttpResponseHeaderCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => pick(_.item, 'key', 'value')),
    [collection, httpId],
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
