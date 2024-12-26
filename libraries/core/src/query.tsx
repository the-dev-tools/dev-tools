import { createClient } from '@connectrpc/connect';
import { createQueryOptions, useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { Struct } from 'effect';
import { useMemo } from 'react';

import {
  QueryListItem,
  QueryListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { queryList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';

import {
  makeGenericDeltaFormTableColumns,
  makeGenericFormTableColumns,
  useDeltaFormTable,
  useFormTable,
} from './form-table';

interface QueryTableProps {
  exampleId: Uint8Array;
}

export const QueryTable = ({ exampleId }: QueryTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const table = useFormTable({
    items,
    schema: QueryListItemSchema,
    columns: makeGenericFormTableColumns<QueryListItem>(),
    onCreate: (_) => requestService.queryCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.queryId),
    onUpdate: (_) => requestService.queryUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.queryDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};

interface QueryDeltaTableProps extends QueryTableProps {
  deltaExampleId: Uint8Array;
}

export const QueryDeltaTable = ({ exampleId, deltaExampleId }: QueryDeltaTableProps) => {
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
    items,
    deltaItems,
    columns: makeGenericDeltaFormTableColumns<QueryListItem>(),
    getParentId: (_) => _.parentQueryId,
    onCreate: (_) => requestService.queryCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.queryId),
    onUpdate: (_) => requestService.queryUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.queryDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};
