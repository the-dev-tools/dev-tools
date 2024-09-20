import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { flexRender } from '@tanstack/react-table';
import { useCallback } from 'react';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Query } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createQuery,
  deleteQuery,
  updateQuery,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';

import { useFormTable } from './form-table';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/')({
  component: Tab,
});

function Tab() {
  const { apiCallId } = Route.useParams();
  const query = useConnectQuery(getApiCall, { id: apiCallId });
  if (!query.isSuccess) return null;
  return <Table data={query.data} />;
}

interface TableProps {
  data: GetApiCallResponse;
}

const Table = ({ data }: TableProps) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { apiCallId } = Route.useParams();

  const { mutateAsync: createMutateAsync } = useConnectMutation(createQuery);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateQuery);
  const { mutate: deleteMutate } = useConnectMutation(deleteQuery);

  const makeItem = useCallback(
    (item?: Partial<Query>) => new Query({ ...item, enabled: true, exampleId: data.example!.meta!.id }),
    [data.example],
  );

  const onCreate = useCallback(({ item }: { item: Query }) => createMutateAsync({ query: item }), [createMutateAsync]);

  const onUpdate = useCallback(({ item }: { item: Query }) => updateMutateAsync({ query: item }), [updateMutateAsync]);

  const onChange = useCallback(
    () => queryClient.invalidateQueries(createQueryOptions(getApiCall, { id: apiCallId }, { transport })),
    [apiCallId, queryClient, transport],
  );

  const table = useFormTable({
    items: data.example!.query,
    makeItem,
    onCreate,
    onUpdate,
    onDelete: deleteMutate,
    onChange,
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
                  style={{
                    width: ((header.getSize() / table.getTotalSize()) * 100).toString() + '%',
                  }}
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
