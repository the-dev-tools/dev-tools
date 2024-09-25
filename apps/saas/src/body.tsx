import { html as cmHtml } from '@codemirror/lang-html';
import { json as cmJson } from '@codemirror/lang-json';
import { xml as cmXml } from '@codemirror/lang-xml';
import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { flexRender } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe } from 'effect';
import { useCallback, useMemo, useState } from 'react';

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
import { getApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { updateExample } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { useFormTable } from './form-table';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId/body')(
  {
    component: Tab,
  },
);

function Tab() {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { apiCallId, exampleId } = Route.useParams();

  const query = useConnectQuery(getApiCall, { id: apiCallId, exampleId });
  const updateMutation = useConnectMutation(updateExample);

  if (!query.isSuccess) return null;
  const body = query.data.example!.body!.value;

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
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

          await queryClient.invalidateQueries(
            createQueryOptions(getApiCall, { id: apiCallId, exampleId }, { transport }),
          );
        }}
      >
        <Radio value='none'>none</Radio>
        <Radio value='forms'>form-data</Radio>
        <Radio value='urlEncodeds'>x-www-form-urlencoded</Radio>
        <Radio value='raw'>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(body),
        Match.when({ case: 'forms' }, ({ value }) => <FormDataTable body={value} />),
        Match.when({ case: 'urlEncodeds' }, ({ value }) => <UrlEncodedTable body={value} />),
        Match.when({ case: 'raw' }, ({ value }) => <RawForm body={value} />),
        Match.orElse(() => null),
      )}
    </div>
  );
}

interface FormDataTableProps {
  body: BodyFormArray;
}

const FormDataTable = ({ body }: FormDataTableProps) => {
  const { exampleId } = Route.useParams();

  const { mutateAsync: createMutateAsync } = useConnectMutation(createBodyForm);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateBodyForm);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyForm);

  const makeItem = useCallback(
    (item?: Partial<BodyFormItem>) => new BodyFormItem({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const table = useFormTable({
    items: body.items,
    makeItem,
    onCreate: createMutateAsync,
    onUpdate: updateMutateAsync,
    onDelete: deleteMutate,
  });

  return (
    <div className='col-span-full rounded border border-black'>
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
  body: BodyUrlEncodedArray;
}

const UrlEncodedTable = ({ body }: UrlEncodedTableProps) => {
  const { exampleId } = Route.useParams();

  const { mutateAsync: createMutateAsync } = useConnectMutation(createBodyUrlEncoded);
  const { mutateAsync: updateMutateAsync } = useConnectMutation(updateBodyUrlEncoded);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyUrlEncoded);

  const makeItem = useCallback(
    (item?: Partial<BodyUrlEncodedItem>) => new BodyUrlEncodedItem({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const table = useFormTable({
    items: body.items,
    makeItem,
    onCreate: createMutateAsync,
    onUpdate: updateMutateAsync,
    onDelete: deleteMutate,
  });

  return (
    <div className='col-span-full rounded border border-black'>
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

const languages = ['text', 'json', 'html', 'xml'] as const;
interface RawFormProps {
  body: BodyRaw;
}

const RawForm = ({ body }: RawFormProps) => {
  const { exampleId } = Route.useParams();

  const updateMutation = useConnectMutation(updateBodyRaw);

  const [value, setValue] = useState(new TextDecoder().decode(body.bodyBytes));
  const [language, setLanguage] = useState<(typeof languages)[number]>('text');

  const extensions = useMemo(
    () =>
      pipe(
        Match.value(language),
        Match.when('text', () => []),
        Match.when('json', () => [cmJson()]),
        Match.when('html', () => [cmHtml()]),
        Match.when('xml', () => [cmXml()]),
        Match.exhaustive,
      ),
    [language],
  );

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
        triggerClassName={tw`px-1.5 py-1`}
        selectedKey={language}
        onSelectionChange={(_) => void setLanguage(_ as (typeof languages)[number])}
      >
        {languages.map((_) => (
          <DropdownItem key={_} id={_}>
            {_}
          </DropdownItem>
        ))}
      </Select>

      <CodeMirror
        value={value}
        onChange={setValue}
        onBlur={() => void updateMutation.mutate({ exampleId, bodyBytes: new TextEncoder().encode(value) })}
        height='100%'
        className='col-span-full self-stretch'
        extensions={extensions}
      />
    </>
  );
};
