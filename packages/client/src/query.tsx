import { pipe } from 'effect';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { QueryDeltaListItem, QueryListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  QueryCreateEndpoint,
  QueryDeleteEndpoint,
  QueryDeltaCreateEndpoint,
  QueryDeltaDeleteEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaMoveEndpoint,
  QueryDeltaResetEndpoint,
  QueryDeltaUpdateEndpoint,
  QueryListEndpoint,
  QueryMoveEndpoint,
  QueryUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { basicReorder, DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { GenericMessage } from '~api/utils';
import { matchAllEndpoint, useQuery } from '~data-client';
import { rootRouteApi } from '~routes';
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
  useFormTableAddRow,
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
  columnReferenceField<GenericMessage<QueryListItem>>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<GenericMessage<QueryListItem>>('value', { allowFiles: true }),
  columnTextField<GenericMessage<QueryListItem>>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const { items } = useQuery(QueryListEndpoint, { exampleId });

  const table = useReactTable({
    columns: dataColumns,
    data: items,
  });

  return <DataTable {...displayTable<GenericMessage<QueryListItem>>()} aria-label='Query items' table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const items: GenericMessage<QueryListItem>[] = useQuery(QueryListEndpoint, { exampleId }).items;

  const formTable = useFormTable<GenericMessage<QueryListItem>>({
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(QueryUpdateEndpoint, item),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New param',
    items,
    onCreate: async () => {
      await dataClient.fetch(QueryCreateEndpoint, { enabled: true, exampleId });
      await dataClient.controller.expireAll({ testKey: matchAllEndpoint(QueryDeltaListEndpoint) });
    },
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: basicReorder(({ position, source, target }) =>
      dataClient.fetch(QueryMoveEndpoint, {
        exampleId,
        position,
        queryId: Ulid.fromCanonical(source).bytes,
        targetQueryId: Ulid.fromCanonical(target).bytes,
      }),
    ),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsCommon<GenericMessage<QueryListItem>>({
          onDelete: (_) => dataClient.fetch(QueryDeleteEndpoint, { queryId: _.queryId }),
        }),
      ]}
      data={items}
      getRowId={(_) => Ulid.construct(_.queryId).toCanonical()}
    >
      {(table) => (
        <DataTable
          {...formTable}
          {...addRow}
          aria-label='Query items'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId: exampleId, exampleId: originId }: DeltaFormTableProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const items = pipe(useQuery(QueryDeltaListEndpoint, { exampleId, originId }).items, (_: QueryDeltaListItem[]) =>
    makeDeltaItems(_, 'queryId'),
  );

  const formTable = useFormTable<GenericMessage<QueryDeltaListItem>>({
    onUpdate: ({ $typeName: _, ...item }) =>
      dataClient.fetch(QueryDeltaUpdateEndpoint, {
        ...item,
        exampleId,
      }),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New param',
    items,
    onCreate: () => dataClient.fetch(QueryDeltaCreateEndpoint, { enabled: true, exampleId, originId }),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: basicReorder(({ position, source, target }) =>
      dataClient.fetch(QueryDeltaMoveEndpoint, {
        exampleId,
        originId,
        position,
        queryId: Ulid.fromCanonical(source).bytes,
        targetQueryId: Ulid.fromCanonical(target).bytes,
      }),
    ),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsDeltaCommon<GenericMessage<QueryDeltaListItem>>({
          onDelete: (_) =>
            dataClient.fetch(QueryDeltaDeleteEndpoint, {
              exampleId,
              queryId: _.queryId,
            }),
          onReset: (_) =>
            dataClient.fetch(QueryDeltaResetEndpoint, {
              exampleId,
              queryId: _.queryId,
            }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => Ulid.construct(_.queryId).toCanonical()}
    >
      {(table) => (
        <DataTable
          {...formTable}
          {...addRow}
          aria-label='Query items'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
