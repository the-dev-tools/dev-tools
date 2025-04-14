import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Struct } from 'effect';
import { useMemo } from 'react';

import {
  QueryListItem,
  QueryListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { queryList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { useConnectSuspenseQuery } from '~/api/connect-query';

import {
  makeGenericDeltaFormTableColumns,
  makeGenericDisplayTableColumns,
  makeGenericFormTableColumns,
  useDeltaFormTable,
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

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const table = useReactTable({
    columns: makeGenericDisplayTableColumns<QueryListItem>(),
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  // eslint-disable-next-line react-compiler/react-compiler
  'use no memo';

  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const table = useFormTable({
    columns: makeGenericFormTableColumns<QueryListItem>(),
    items,
    onCreate: (_) => requestService.queryCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.queryId),
    onDelete: (_) => requestService.queryDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.queryUpdate(Struct.omit(_, '$typeName')),
    schema: QueryListItemSchema,
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
      createQueryOptions(queryList, { exampleId }, { transport }),
      createQueryOptions(queryList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const table = useDeltaFormTable({
    columns: makeGenericDeltaFormTableColumns<QueryListItem>(),
    deltaItems,
    getParentId: (_) => _.parentQueryId!,
    items,
    onCreate: (_) =>
      requestService
        .queryCreate({
          ...Struct.omit(_, '$typeName'),
          exampleId: deltaExampleId,
          parentQueryId: _.queryId,
        })
        .then((_) => _.queryId),
    onDelete: (_) => requestService.queryDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.queryUpdate(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};
