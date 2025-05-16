import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';

import {
  HeaderCreateEndpoint,
  HeaderDeleteEndpoint,
  HeaderListEndpoint,
  HeaderUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { HeaderListItemEntity } from '@the-dev-tools/spec/meta/collection/item/request/v1/request.entities.ts';
import { DataTable } from '@the-dev-tools/ui/data-table';

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
  columnCheckboxField<HeaderListItemEntity>('enabled', { meta: { divider: false } }),
  columnReferenceField<HeaderListItemEntity>('key'),
  columnReferenceField<HeaderListItemEntity>('value'),
  columnTextField<HeaderListItemEntity>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const transport = useTransport();

  const { items } = useSuspense(HeaderListEndpoint, transport, { exampleId });

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
  const transport = useTransport();
  const controller = useController();

  const { items } = useSuspense(HeaderListEndpoint, transport, { exampleId });

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActions<HeaderListItemEntity>({
        cell: ({ row }) => (
          <ColumnActionDelete
            onAction={() => controller.fetch(HeaderDeleteEndpoint, transport, { headerId: row.original.headerId })}
          />
        ),
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New header',
    items,
    onCreate: () => controller.fetch(HeaderCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(HeaderUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} />;
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId, exampleId }: DeltaFormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  // TODO: fetch in parallel
  const { items: itemsBase } = useSuspense(HeaderListEndpoint, transport, { exampleId });
  const { items: itemsDelta } = useSuspense(HeaderListEndpoint, transport, { exampleId: deltaExampleId });

  const items = makeDeltaItems({
    getId: (_) => _.headerId.toString(),
    getParentId: (_) => _.parentHeaderId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = deltaFormTable<HeaderListItemEntity>({
    getParentId: (_) => _.parentHeaderId?.toString(),
    onCreate: ({ $typeName: _, headerId, ...item }) =>
      controller.fetch(HeaderCreateEndpoint, transport, {
        ...item,
        exampleId: deltaExampleId,
        parentHeaderId: headerId,
      }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(HeaderUpdateEndpoint, transport, item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActions<HeaderListItemEntity>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentHeaderId !== undefined}
              onAction={() => controller.fetch(HeaderDeleteEndpoint, transport, { headerId: row.original.headerId })}
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
