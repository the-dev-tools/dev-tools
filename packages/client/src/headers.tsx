import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';

import { HeaderListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  headerCreate,
  headerDelete,
  headerList,
  headerUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { useConnectMutation, useConnectSuspenseQuery } from '~/api/connect-query';

import {
  ColumnActionDelete,
  columnActions,
  ColumnActionUndoDelta,
  columnCheckboxField,
  columnTextField,
  columnTextFieldWithReference,
  displayTable,
  ReactTableNoMemo,
  useDeltaFormTable,
  useDeltaItems,
  useFormTable,
} from './form-table';

interface HeaderTableProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const HeaderTable = ({ deltaExampleId, exampleId, isReadOnly }: HeaderTableProps) => {
  if (isReadOnly) return <DisplayTable exampleId={exampleId} />;
  if (deltaExampleId) return <DeltaFormTable deltaExampleId={deltaExampleId} exampleId={exampleId} />;
  return <FormTable exampleId={exampleId} />;
};

const dataColumns = [
  columnCheckboxField<HeaderListItem>('enabled', { meta: { divider: false } }),
  columnTextFieldWithReference<HeaderListItem>('key'),
  columnTextFieldWithReference<HeaderListItem>('value'),
  columnTextField<HeaderListItem>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(headerList, { exampleId });

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
  } = useConnectSuspenseQuery(headerList, { exampleId });

  const { mutateAsync: create } = useConnectMutation(headerCreate);
  const { mutateAsync: update } = useConnectMutation(headerUpdate);

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActions<HeaderListItem>({
        cell: ({ row }) => <ColumnActionDelete input={{ headerId: row.original.headerId }} schema={headerDelete} />,
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New header',
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

  const { mutateAsync: create } = useConnectMutation(headerCreate);
  const { mutateAsync: update } = useConnectMutation(headerUpdate);

  const [
    {
      data: { items: itemsBase },
    },
    {
      data: { items: itemsDelta },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(headerList, { exampleId }, { transport }),
      createQueryOptions(headerList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const items = useDeltaItems({
    getId: (_) => _.headerId.toString(),
    getParentId: (_) => _.parentHeaderId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = useDeltaFormTable<HeaderListItem>({
    getParentId: (_) => _.parentHeaderId?.toString(),
    onCreate: ({ $typeName: _, headerId, ...item }) =>
      create({ ...item, exampleId: deltaExampleId, parentHeaderId: headerId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActions<HeaderListItem>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentHeaderId !== undefined}
              input={{ headerId: row.original.headerId }}
              schema={headerDelete}
            />
          ),
        }),
      ]}
      data={items}
      getCoreRowModel={getCoreRowModel()}
      getRowId={(_) => (_.parentHeaderId ?? _.headerId).toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};
