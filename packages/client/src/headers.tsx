import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';

import { HeaderDeltaListItem, HeaderListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  HeaderCreateEndpoint,
  HeaderDeltaCreateEndpoint,
  HeaderDeltaDeleteEndpoint,
  HeaderDeltaListEndpoint,
  HeaderDeltaResetEndpoint,
  HeaderListEndpoint,
  HeaderUpdateEndpoint,
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
  columnReferenceField<GenericMessage<HeaderListItem>>('value'),
  columnTextField<GenericMessage<HeaderListItem>>('description', { meta: { divider: false } }),
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
  });

  return <DataTable {...displayTable} table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items: GenericMessage<HeaderListItem>[] = useSuspense(HeaderListEndpoint, transport, { exampleId }).items;

  const table = useReactTable({
    columns: [
      ...dataColumns,
      columnActionsCommon<GenericMessage<HeaderListItem>>({
        onDelete: (_) => controller.fetch(HeaderDeltaDeleteEndpoint, transport, { headerId: _.headerId }),
      }),
    ],
    data: items,
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

  const items: GenericMessage<HeaderDeltaListItem>[] = useSuspense(HeaderDeltaListEndpoint, transport, {
    exampleId: deltaExampleId,
    originId: exampleId,
  }).items;

  const formTable = useFormTable({
    createLabel: 'New header',
    items,
    onCreate: () => controller.fetch(HeaderDeltaCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(HeaderDeltaDeleteEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsDeltaCommon<GenericMessage<HeaderDeltaListItem>>({
          onDelete: (_) => controller.fetch(HeaderDeltaDeleteEndpoint, transport, { headerId: _.headerId }),
          onReset: (_) => controller.fetch(HeaderDeltaResetEndpoint, transport, { headerId: _.headerId }),
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
