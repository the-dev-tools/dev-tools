import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Struct } from 'effect';
import { useMemo } from 'react';

import {
  HeaderListItem,
  HeaderListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { headerList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { useConnectSuspenseQuery } from '~/api/connect-query';

import {
  makeGenericDeltaFormTableColumns,
  makeGenericDisplayTableColumns,
  makeGenericFormTableColumns,
  useDeltaFormTable,
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
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(headerList, { exampleId });

  const table = useFormTable({
    columns: makeGenericFormTableColumns<HeaderListItem>(),
    items,
    onCreate: (_) => requestService.headerCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.headerId),
    onDelete: (_) => requestService.headerDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.headerUpdate(Struct.omit(_, '$typeName')),
    schema: HeaderListItemSchema,
  });

  return <DataTable table={table} />;
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
