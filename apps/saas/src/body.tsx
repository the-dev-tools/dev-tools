import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { flexRender } from '@tanstack/react-table';
import { Match, pipe } from 'effect';
import { useCallback, useState } from 'react';

import {
  Body,
  BodyFormArray,
  BodyFormItem,
  BodyRaw,
  BodyUrlEncodedArray,
  BodyUrlEncodedItem,
} from '@the-dev-tools/protobuf/body/v1/body_pb';
import {
  createBodyForm,
  createBodyUrlEncoded,
  deleteBodyForm,
  deleteBodyUrlEncoded,
  updateBodyForm,
  updateBodyRaw,
  updateBodyUrlEncoded,
} from '@the-dev-tools/protobuf/body/v1/body-BodyService_connectquery';
import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { updateExample } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextAreaField } from '@the-dev-tools/ui/text-field';

import { useFormTable } from './form-table';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/body')({
  component: Tab,
});

function Tab() {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { apiCallId } = Route.useParams();

  const query = useConnectQuery(getApiCall, { id: apiCallId });
  const updateMutation = useConnectMutation(updateExample);

  if (!query.isSuccess) return null;
  const body = query.data.example!.body!.value;

  return (
    <>
      <RadioGroup
        aria-label='Body type'
        orientation='horizontal'
        value={body.case ?? 'none'}
        onChange={async (kind) => {
          await updateMutation.mutateAsync({
            id: query.data.example!.meta!.id,
            bodyType: new Body({
              value: {
                case: kind as Exclude<Body['value']['case'], undefined>,
                value: {},
              },
            }),
          });

          await queryClient.invalidateQueries(createQueryOptions(getApiCall, { id: apiCallId }, { transport }));
        }}
      >
        <Radio value='none'>none</Radio>
        <Radio value='forms'>form-data</Radio>
        <Radio value='urlEncodeds'>x-www-form-urlencoded</Radio>
        <Radio value='raw'>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(body),
        Match.when({ case: 'forms' }, ({ value }) => <FormDataTable data={query.data} body={value} />),
        Match.when({ case: 'urlEncodeds' }, ({ value }) => <UrlEncodedTable data={query.data} body={value} />),
        Match.when({ case: 'raw' }, ({ value }) => <RawForm data={query.data} body={value} />),
        Match.orElse(() => null),
      )}
    </>
  );
}

interface FormDataTableProps {
  data: GetApiCallResponse;
  body: BodyFormArray;
}

const FormDataTable = ({ data, body }: FormDataTableProps) => {
  const { mutateAsync: createMutateAsync } = useConnectMutation(createBodyForm);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateBodyForm);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyForm);

  const makeItem = useCallback(
    (item?: Partial<BodyFormItem>) => new BodyFormItem({ ...item, enabled: true, exampleId: data.example!.meta!.id }),
    [data.example],
  );

  const table = useFormTable({
    items: body.items,
    makeItem,
    onCreate: createMutateAsync,
    onUpdate: updateMutateAsync,
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

interface UrlEncodedTableProps {
  data: GetApiCallResponse;
  body: BodyUrlEncodedArray;
}

const UrlEncodedTable = ({ data, body }: UrlEncodedTableProps) => {
  const { mutateAsync: createMutateAsync } = useConnectMutation(createBodyUrlEncoded);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateBodyUrlEncoded);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyUrlEncoded);

  const makeItem = useCallback(
    (item?: Partial<BodyUrlEncodedItem>) =>
      new BodyUrlEncodedItem({ ...item, enabled: true, exampleId: data.example!.meta!.id }),
    [data.example],
  );

  const table = useFormTable({
    items: body.items,
    makeItem,
    onCreate: createMutateAsync,
    onUpdate: updateMutateAsync,
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

interface RawFormProps {
  data: GetApiCallResponse;
  body: BodyRaw;
}

const RawForm = ({ data, body }: RawFormProps) => {
  const [value, setValue] = useState(new TextDecoder().decode(body.bodyBytes));

  const updateMutation = useConnectMutation(updateBodyRaw);

  return (
    <TextAreaField
      aria-label='Raw body value'
      value={value}
      onChange={setValue}
      onBlur={() =>
        void updateMutation.mutate({
          exampleId: data.example!.meta!.id,
          bodyBytes: new TextEncoder().encode(value),
        })
      }
      className='h-full'
      areaClassName={tw`h-full`}
    />
  );
};
