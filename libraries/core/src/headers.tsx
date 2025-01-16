import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { Struct } from 'effect';
import { useMemo } from 'react';

import { useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import {
  HeaderListItem,
  HeaderListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { headerList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';

import {
  makeGenericDeltaFormTableColumns,
  makeGenericFormTableColumns,
  useDeltaFormTable,
  useFormTable,
} from './form-table';

interface HeaderTableProps {
  exampleId: Uint8Array;
}

export const HeaderTable = ({ exampleId }: HeaderTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(headerList, { exampleId });

  const table = useFormTable({
    items,
    schema: HeaderListItemSchema,
    columns: makeGenericFormTableColumns<HeaderListItem>(),
    onCreate: (_) => requestService.headerCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.headerId),
    onUpdate: (_) => requestService.headerUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.headerDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};

interface HeaderDeltaTableProps extends HeaderTableProps {
  deltaExampleId: Uint8Array;
}

export const HeaderDeltaTable = ({ exampleId, deltaExampleId }: HeaderDeltaTableProps) => {
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
    items,
    deltaItems,
    columns: makeGenericDeltaFormTableColumns<HeaderListItem>(),
    getParentId: (_) => _.parentHeaderId!,
    onCreate: (_) => requestService.headerCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.headerId),
    onUpdate: (_) => requestService.headerUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.headerDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};
