import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Match, pipe, Struct } from 'effect';
import { useCallback, useMemo, useState } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';

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
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

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
  const queryClient = useQueryClient();

  const { workspaceId, apiCallId, exampleId } = Route.useParams();

  const createMutation = useConnectMutation(createBodyForm);
  const updateMutation = useConnectMutation(updateBodyForm);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyForm);

  const makeItem = useCallback(
    (item?: Partial<BodyFormItem>) => new BodyFormItem({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const values = useMemo(() => ({ items: [...body.items, makeItem()] }), [body.items, makeItem]);
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({ control: form.control, name: 'items' });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<BodyFormItem>();
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
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, workspaceId, deleteMutate, getValues, removeField]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: Struct.get('id'),
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(() => {
    const items = Array.dropRight(getValues('items'), 1);
    queryClient.setQueryData(
      createConnectQueryKey(getApiCall, { id: apiCallId, exampleId }),
      createProtobufSafeUpdater(getApiCall, (old) => ({
        ...old,
        example: {
          ...old?.example,
          body: new Body({ value: { case: 'forms', value: { items } } }),
        },
      })),
    );
  }, [apiCallId, exampleId, getValues, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    onCreate: async (item) => (await createMutation.mutateAsync({ item })).id,
    onUpdate: (item) => updateMutation.mutateAsync({ item }),
    setData,
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
  const queryClient = useQueryClient();

  const { workspaceId, apiCallId, exampleId } = Route.useParams();

  const createMutation = useConnectMutation(createBodyUrlEncoded);
  const updateMutation = useConnectMutation(updateBodyUrlEncoded);
  const { mutate: deleteMutate } = useConnectMutation(deleteBodyUrlEncoded);

  const makeItem = useCallback(
    (item?: Partial<BodyUrlEncodedItem>) => new BodyUrlEncodedItem({ ...item, enabled: true, exampleId }),
    [exampleId],
  );

  const values = useMemo(() => ({ items: [...body.items, makeItem()] }), [body.items, makeItem]);
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({ control: form.control, name: 'items' });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<BodyUrlEncodedItem>();
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
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, workspaceId, deleteMutate, getValues, removeField]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: Struct.get('id'),
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(() => {
    const items = Array.dropRight(getValues('items'), 1);
    queryClient.setQueryData(
      createConnectQueryKey(getApiCall, { id: apiCallId, exampleId }),
      createProtobufSafeUpdater(getApiCall, (old) => ({
        ...old,
        example: {
          ...old?.example,
          body: new Body({ value: { case: 'forms', value: { items } } }),
        },
      })),
    );
  }, [apiCallId, exampleId, getValues, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    onCreate: async (item) => (await createMutation.mutateAsync({ item })).id,
    onUpdate: (item) => updateMutation.mutateAsync({ item }),
    setData,
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

  const { data: extensions } = useQuery({
    initialData: [],
    queryKey: ['code-mirror', language],
    queryFn: async () => {
      if (language === 'text') return [];
      return await pipe(
        Match.value(language),
        Match.when('json', () => import('@codemirror/lang-json').then((_) => _.json())),
        Match.when('html', () => import('@codemirror/lang-html').then((_) => _.html())),
        Match.when('xml', () => import('@codemirror/lang-xml').then((_) => _.xml())),
        Match.exhaustive,
        (_) => _.then(Array.make),
      );
    },
  });

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
