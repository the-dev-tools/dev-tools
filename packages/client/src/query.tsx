import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';

import {
  QueryCreateEndpoint,
  QueryDeleteEndpoint,
  QueryListEndpoint,
  QueryUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { QueryListItemEntity } from '@the-dev-tools/spec/meta/collection/item/request/v1/request.entities.ts';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';

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
  columnCheckboxField<QueryListItemEntity>('enabled', { meta: { divider: false } }),
  columnReferenceField<QueryListItemEntity>('key'),
  columnReferenceField<QueryListItemEntity>('value'),
  columnTextField<QueryListItemEntity>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const transport = useTransport();

  const { items } = useSuspense(QueryListEndpoint, transport, { exampleId });

  const table = useReactTable({
    columns: dataColumns,
    data: items,
  });

  return <DataTable {...displayTable} table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const { items } = useSuspense(QueryListEndpoint, transport, { exampleId });

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActions<QueryListItemEntity>({
        cell: ({ row }) => (
          <ColumnActionDelete
            onAction={() => controller.fetch(QueryDeleteEndpoint, transport, { queryId: row.original.queryId })}
          />
        ),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: () => controller.fetch(QueryCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(QueryUpdateEndpoint, transport, item),
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
  const { items: itemsBase } = useSuspense(QueryListEndpoint, transport, { exampleId });
  const { items: itemsDelta } = useSuspense(QueryListEndpoint, transport, { exampleId: deltaExampleId });

  const items = makeDeltaItems({
    getId: (_) => _.queryId.toString(),
    getParentId: (_) => _.parentQueryId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = deltaFormTable<QueryListItemEntity>({
    getParentId: (_) => _.parentQueryId?.toString(),
    onCreate: ({ $typeName: _, queryId, ...item }) =>
      controller.fetch(QueryCreateEndpoint, transport, { ...item, exampleId: deltaExampleId, parentQueryId: queryId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(QueryUpdateEndpoint, transport, item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActions<QueryListItemEntity>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentQueryId !== undefined}
              onAction={() => controller.fetch(QueryDeleteEndpoint, transport, { queryId: row.original.queryId })}
            />
          ),
        }),
      ]}
      data={items}
      getRowId={(_) => (_.parentQueryId ?? _.queryId).toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};
