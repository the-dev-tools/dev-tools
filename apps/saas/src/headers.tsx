import { useMutation as useConnectMutation, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { useDebouncedCallback } from 'use-debounce';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Header } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createHeader,
  deleteHeader,
  updateHeader,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/headers')({
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
  const updateMutation = useConnectMutation(updateHeader);
  const createMutation = useConnectMutation(createHeader);
  const { mutate: delete_ } = useConnectMutation(deleteHeader);

  const makeTemplateHeader = useCallback(
    () => new Header({ enabled: true, exampleId: data.example!.meta!.id }),
    [data.example],
  );

  const values = useMemo(
    () => ({ header: [...data.example!.header, makeTemplateHeader()] }),
    [data.example, makeTemplateHeader],
  );

  const { getValues, ...form } = useForm({ values });
  const { fields, remove: removeField, ...fieldArray } = useFieldArray({ name: 'header', control: form.control });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<Header>();
    return [
      accessor('enabled', {
        header: '',
        minSize: 0,
        size: 0,
        cell: ({ row, table }) => {
          if (row.index + 1 === table.getRowCount()) return null;
          return (
            <CheckboxRHF key={row.id} control={form.control} name={`header.${row.index}.enabled`} className='p-1' />
          );
        },
      }),
      accessor('key', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`header.${row.index}.key`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('value', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`header.${row.index}.value`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`header.${row.index}.description`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        minSize: 0,
        size: 0,
        cell: ({ row, table }) => {
          if (row.index + 1 === table.getRowCount()) return null;

          return (
            <Button
              className='text-red-700'
              kind='placeholder'
              variant='placeholder ghost'
              onPress={() => {
                const id = getValues(`header.${row.index}.id`);
                delete_({ id });
                removeField(row.index);
              }}
            >
              <LuTrash2 />
            </Button>
          );
        },
      }),
    ];
  }, [delete_, form.control, getValues, removeField]);

  const table = useReactTable({
    columns,
    data: fields,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.id,
  });

  const updateHeaderQueueMap = useRef(new Map<string, Header>());
  const updateHeaders = useDebouncedCallback(() => {
    const updates = updateHeaderQueueMap.current;
    updates.forEach(async (header) => {
      updates.delete(header.id); // Un-queue update
      if (header.id) {
        await updateMutation.mutateAsync({ header });
      } else {
        const { id } = await createMutation.mutateAsync({ header });
        const index = getValues('header').length - 1;

        form.setValue(`header.${index}`, new Header({ ...header, id }));
        updates.delete(id); // Delete update that gets queued by setting new id

        fieldArray.append(makeTemplateHeader(), { shouldFocus: false });

        // Redirect outdated queued update to the new id
        const outdated = updates.get('');
        if (outdated !== undefined) {
          updates.delete('');
          updates.set(id, new Header({ ...outdated, id }));
        }
      }
    });
  }, 500);

  useEffect(() => {
    const watch = form.watch((_, { name }) => {
      const rowName = name?.match(/(^header.[\d]+)/g)?.[0] as `header.${number}` | undefined;
      if (!rowName) return;
      const rowValues = getValues(rowName);
      updateHeaderQueueMap.current.set(rowValues.id, rowValues);
      void updateHeaders();
    });
    return () => void watch.unsubscribe();
  }, [form, getValues, updateHeaders]);

  useEffect(() => () => void updateHeaders.flush(), [updateHeaders]);

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
