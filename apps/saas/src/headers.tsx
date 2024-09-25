import { useMutation as useConnectMutation, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute } from '@tanstack/react-router';
import { flexRender } from '@tanstack/react-table';
import { useCallback } from 'react';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Header } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createHeader,
  deleteHeader,
  updateHeader,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';

import { useFormTable } from './form-table';

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId/headers',
)({
  component: Tab,
});

function Tab() {
  const { apiCallId, exampleId } = Route.useParams();
  const query = useConnectQuery(getApiCall, { id: apiCallId, exampleId });
  if (!query.isSuccess) return null;
  return <Table data={query.data} />;
}

interface TableProps {
  data: GetApiCallResponse;
}

const Table = ({ data }: TableProps) => {
  const { exampleId } = Route.useParams();

  const { mutateAsync: createMutateAsync } = useConnectMutation(createHeader);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateHeader);
  const { mutate: deleteMutate } = useConnectMutation(deleteHeader);

  const makeItem = useCallback(
    (item?: Partial<Header>) => new Header({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const onCreate = useCallback(
    ({ item }: { item: Header }) => createMutateAsync({ header: item }),
    [createMutateAsync],
  );

  const onUpdate = useCallback(
    ({ item }: { item: Header }) => updateMutateAsync({ header: item }),
    [updateMutateAsync],
  );

  const table = useFormTable({
    items: data.example!.header,
    makeItem,
    onCreate,
    onUpdate,
    onDelete: deleteMutate,
  });

  return (
    <div className='rounded border border-black'>
      <table className='w-full divide-inherit border-inherit'>
        <thead className='divide-y divide-inherit border-b border-inherit'>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className='p-1.5 text-left text-sm font-normal capitalize text-neutral-500'
                  style={{ width: ((header.getSize() / table.getTotalSize()) * 100).toString() + '%' }}
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className='divide-y divide-inherit'>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className='break-all align-middle text-sm'>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
