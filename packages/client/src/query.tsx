import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';
import { pipe } from 'effect';

import { QueryDeltaListItem, QueryListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  QueryCreateEndpoint,
  QueryDeleteEndpoint,
  QueryDeltaCreateEndpoint,
  QueryDeltaDeleteEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaResetEndpoint,
  QueryDeltaUpdateEndpoint,
  QueryListEndpoint,
  QueryUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { GenericMessage } from '~api/utils';

import {
  columnActionsCommon,
  columnActionsDeltaCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
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
  columnCheckboxField<GenericMessage<QueryListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<QueryListItem>>('key'),
  columnReferenceField<GenericMessage<QueryListItem>>('value'),
  columnTextField<GenericMessage<QueryListItem>>('description', { meta: { divider: false } }),
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

  const items: GenericMessage<QueryListItem>[] = useSuspense(QueryListEndpoint, transport, { exampleId }).items;

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActionsCommon<GenericMessage<QueryListItem>>({
        onDelete: (_) => controller.fetch(QueryDeleteEndpoint, transport, { queryId: _.queryId }),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: async () => {
      await controller.fetch(QueryCreateEndpoint, transport, { enabled: true, exampleId });
      await controller.invalidateAll({ testKey: (_) => _.startsWith(QueryDeltaListEndpoint.name) });
    },
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(QueryUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} />;
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId: exampleId, exampleId: originId }: DeltaFormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items = pipe(
    useSuspense(QueryDeltaListEndpoint, transport, { exampleId, originId }).items,
    (_: QueryDeltaListItem[]) => makeDeltaItems(_, 'queryId'),
  );

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: () => controller.fetch(QueryDeltaCreateEndpoint, transport, { enabled: true, exampleId, originId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(QueryDeltaUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsDeltaCommon<GenericMessage<QueryDeltaListItem>>({
          onDelete: (_) => controller.fetch(QueryDeltaDeleteEndpoint, transport, { queryId: _.queryId }),
          onReset: (_) => controller.fetch(QueryDeltaResetEndpoint, transport, { queryId: _.queryId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.queryId.toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};
