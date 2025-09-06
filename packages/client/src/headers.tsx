import { pipe } from 'effect';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { HeaderDeltaListItem, HeaderListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  HeaderCreateEndpoint,
  HeaderDeltaCreateEndpoint,
  HeaderDeltaDeleteEndpoint,
  HeaderDeltaListEndpoint,
  HeaderDeltaMoveEndpoint,
  HeaderDeltaResetEndpoint,
  HeaderDeltaUpdateEndpoint,
  HeaderListEndpoint,
  HeaderMoveEndpoint,
  HeaderUpdateEndpoint,
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
  columnCheckboxField<GenericMessage<HeaderListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<HeaderListItem>>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<GenericMessage<HeaderListItem>>('value', { allowFiles: true }),
  columnTextField<GenericMessage<HeaderListItem>>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const { items } = useQuery(HeaderListEndpoint, { exampleId });

  const table = useReactTable({
    columns: dataColumns,
    data: items,
  });

  return <DataTable {...displayTable<GenericMessage<HeaderListItem>>()} aria-label='Headers' table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const items: GenericMessage<HeaderListItem>[] = useQuery(HeaderListEndpoint, { exampleId }).items;

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActionsCommon<GenericMessage<HeaderListItem>>({
        onDelete: (_) => dataClient.fetch(HeaderDeltaDeleteEndpoint, { headerId: _.headerId }),
      }),
    ],
    data: items,
    getRowId: (_) => Ulid.construct(_.headerId).toCanonical(),
  });

  const formTable = useFormTable({
    createLabel: 'New header',
    items,
    onCreate: async () => {
      await dataClient.fetch(HeaderCreateEndpoint, { enabled: true, exampleId });
      await dataClient.controller.expireAll({ testKey: matchAllEndpoint(HeaderDeltaListEndpoint) });
    },
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(HeaderUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: basicReorder(({ position, source, target }) =>
      dataClient.fetch(HeaderMoveEndpoint, {
        exampleId,
        headerId: Ulid.fromCanonical(source).bytes,
        position,
        targetHeaderId: Ulid.fromCanonical(target).bytes,
      }),
    ),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return <DataTable {...formTable} aria-label='Headers' dragAndDropHooks={dragAndDropHooks} table={table} />;
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId: exampleId, exampleId: originId }: DeltaFormTableProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const items = pipe(useQuery(HeaderDeltaListEndpoint, { exampleId, originId }).items, (_: HeaderDeltaListItem[]) =>
    makeDeltaItems(_, 'headerId'),
  );

  const formTable = useFormTable({
    createLabel: 'New header',
    items,
    onCreate: () => dataClient.fetch(HeaderDeltaCreateEndpoint, { enabled: true, exampleId, originId }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(HeaderDeltaUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: basicReorder(({ position, source, target }) =>
      dataClient.fetch(HeaderDeltaMoveEndpoint, {
        exampleId,
        headerId: Ulid.fromCanonical(source).bytes,
        originId,
        position,
        targetHeaderId: Ulid.fromCanonical(target).bytes,
      }),
    ),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsDeltaCommon<GenericMessage<HeaderDeltaListItem>>({
          onDelete: (_) => dataClient.fetch(HeaderDeltaDeleteEndpoint, { headerId: _.headerId }),
          onReset: (_) => dataClient.fetch(HeaderDeltaResetEndpoint, { headerId: _.headerId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => Ulid.construct(_.headerId).toCanonical()}
    >
      {(table) => (
        <DataTable {...formTable} aria-label='Headers' dragAndDropHooks={dragAndDropHooks} table={table} />
      )}
    </ReactTableNoMemo>
  );
};
