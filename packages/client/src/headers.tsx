import { useRouteContext } from '@tanstack/react-router';
import { pipe } from 'effect';

import { HeaderDeltaListItem, HeaderListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  HeaderCreateEndpoint,
  HeaderDeltaCreateEndpoint,
  HeaderDeltaDeleteEndpoint,
  HeaderDeltaListEndpoint,
  HeaderDeltaResetEndpoint,
  HeaderDeltaUpdateEndpoint,
  HeaderListEndpoint,
  HeaderUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { GenericMessage } from '~api/utils';
import { useQuery } from '~data-client';

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
  columnReferenceField<GenericMessage<HeaderListItem>>('key'),
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

  return <DataTable {...displayTable} table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items: GenericMessage<HeaderListItem>[] = useQuery(HeaderListEndpoint, { exampleId }).items;

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActionsCommon<GenericMessage<HeaderListItem>>({
        onDelete: (_) => dataClient.fetch(HeaderDeltaDeleteEndpoint, { headerId: _.headerId }),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New header',
    items,
    onCreate: async () => {
      await dataClient.fetch(HeaderCreateEndpoint, { enabled: true, exampleId });
      // TODO: improve key matching
      await dataClient.controller.expireAll({ testKey: (_) => _.startsWith(`["${HeaderDeltaListEndpoint.name}"`) });
    },
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(HeaderUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} />;
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId: exampleId, exampleId: originId }: DeltaFormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

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
      getRowId={(_) => _.headerId.toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};
