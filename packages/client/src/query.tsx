import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';

import { QueryListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  queryCreate,
  queryDelete,
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { useConnectMutation, useConnectSuspenseQuery } from '~/api/connect-query';

import {
  ColumnActionDelete,
  columnActions,
  ColumnActionUndoDelta,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  deltaFormTable,
  displayTable,
  makeDeltaItems,
  ReactTableNoMemo,
  useFormTable,
} from './form-table';

interface QueryTableProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const QueryTable = ({ deltaExampleId, exampleId, isReadOnly }: QueryTableProps) => {
  if (isReadOnly) return <DisplayTable exampleId={exampleId} />;
  if (deltaExampleId) return <DeltaFormTable deltaExampleId={deltaExampleId} exampleId={exampleId} />;
  return <FormTable exampleId={exampleId} />;
};

const dataColumns = [
  columnCheckboxField<QueryListItem>('enabled', { meta: { divider: false } }),
  columnReferenceField<QueryListItem>('key'),
  columnReferenceField<QueryListItem>('value'),
  columnTextField<QueryListItem>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const table = useReactTable({
    columns: dataColumns,
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable {...displayTable} table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const { mutateAsync: create } = useConnectMutation(queryCreate);
  const { mutateAsync: update } = useConnectMutation(queryUpdate);

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActions<QueryListItem>({
        cell: ({ row }) => <ColumnActionDelete input={{ queryId: row.original.queryId }} schema={queryDelete} />,
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: () => create({ enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} />;
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId, exampleId }: DeltaFormTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const { mutateAsync: create } = useConnectMutation(queryCreate);
  const { mutateAsync: update } = useConnectMutation(queryUpdate);

  const [
    {
      data: { items: itemsBase },
    },
    {
      data: { items: itemsDelta },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(queryList, { exampleId }, { transport }),
      createQueryOptions(queryList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const items = makeDeltaItems({
    getId: (_) => _.queryId.toString(),
    getParentId: (_) => _.parentQueryId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = deltaFormTable<QueryListItem>({
    getParentId: (_) => _.parentQueryId?.toString(),
    onCreate: ({ $typeName: _, queryId, ...item }) =>
      create({ ...item, exampleId: deltaExampleId, parentQueryId: queryId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActions<QueryListItem>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentQueryId !== undefined}
              input={{ queryId: row.original.queryId }}
              schema={queryDelete}
            />
          ),
        }),
      ]}
      data={items}
      getCoreRowModel={getCoreRowModel()}
      getRowId={(_) => (_.parentQueryId ?? _.queryId).toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};
