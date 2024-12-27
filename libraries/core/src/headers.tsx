import { createClient } from '@connectrpc/connect';
import { useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { useRouteContext } from '@tanstack/react-router';
import { Struct } from 'effect';
import { useMemo } from 'react';

import {
  HeaderListItem,
  HeaderListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { headerList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';

import { makeGenericFormTableColumns, useFormTable } from './form-table';

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
