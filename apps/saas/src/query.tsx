import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, Struct } from 'effect';
import { useCallback, useMemo } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Query } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createQuery,
  deleteQuery,
  updateQuery,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId/')({
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
  const queryClient = useQueryClient();

  const { workspaceId, apiCallId, exampleId } = Route.useParams();

  const createMutation = useConnectMutation(createQuery);
  const updateMutation = useConnectMutation(updateQuery);
  const { mutate: deleteMutate } = useConnectMutation(deleteQuery);

  const makeItem = useCallback(
    (item?: Partial<Query>) => new Query({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const values = useMemo(() => ({ items: [...data.example!.query, makeItem()] }), [data.example, makeItem]);
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({ control: form.control, name: 'items' });

  const setData = useCallback(() => {
    const query = Array.dropRight(getValues('items'), 1);
    queryClient.setQueryData(
      createConnectQueryKey(getApiCall, { id: apiCallId, exampleId }),
      createProtobufSafeUpdater(getApiCall, (old) => ({ ...old, example: { ...old?.example, query } })),
    );
  }, [apiCallId, exampleId, getValues, queryClient]);

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<Query>();
    return [
      accessor('enabled', {
        header: '',
        size: 0,
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table}>
            <CheckboxRHF control={form.control} name={`items.${row.index}.enabled`} variant='table-cell' />
          </HidePlaceholderCell>
        ),
      }),
      accessor('key', {
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.key`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('value', {
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.value`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF control={form.control} name={`items.${row.index}.description`} variant='table-cell' />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        size: 0,
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table}>
            <Button
              className='text-red-700'
              kind='placeholder'
              variant='placeholder ghost'
              onPress={() => {
                deleteMutate({ id: getValues(`items.${row.index}.id`) });
                removeField(row.index);
                void setData();
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, workspaceId, deleteMutate, getValues, removeField, setData]);

  const table = useReactTable<Query>({
    getCoreRowModel: getCoreRowModel(),
    getRowId: Struct.get('id'),
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    onCreate: async (query) => (await createMutation.mutateAsync({ query })).id,
    onUpdate: (query) => updateMutation.mutateAsync({ query }),
    onChange: setData,
    setData,
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
