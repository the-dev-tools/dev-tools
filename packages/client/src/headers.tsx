import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Struct } from 'effect';
import { useMemo } from 'react';

import {
  HeaderListItem,
  RequestService
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
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
  columnCheckboxField,
  columnTextField,
  columnTextFieldWithReference,
  makeGenericDeltaFormTableColumns,
  makeGenericDisplayTableColumns,
  useDeltaFormTable,
  useFormTable
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

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(headerList, { exampleId });

  const table = useReactTable({
    columns: makeGenericDisplayTableColumns<HeaderListItem>(),
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} />;
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
      columnCheckboxField<HeaderListItem>('enabled', { meta: { divider: false } }),
      columnTextFieldWithReference<HeaderListItem>('key'),
      columnTextFieldWithReference<HeaderListItem>('value'),
      columnTextField<HeaderListItem>('description', { meta: { divider: false } }),
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
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const [
    {
      data: { items },
    },
    {
      data: { items: deltaItems },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(headerList, { exampleId }, { transport }),
      createQueryOptions(headerList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const table = useDeltaFormTable({
    columns: makeGenericDeltaFormTableColumns<HeaderListItem>(),
    deltaItems,
    getParentId: (_) => _.parentHeaderId!,
    items,
    onCreate: (_) =>
      requestService
        .headerCreate({
          ...Struct.omit(_, '$typeName'),
          exampleId: deltaExampleId,
          parentHeaderId: _.headerId,
        })
        .then((_) => _.headerId),
    onDelete: (_) => requestService.headerDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.headerUpdate(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};
