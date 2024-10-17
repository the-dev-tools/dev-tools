import { create, fromJson, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Match, pipe } from 'effect';
import { useCallback, useMemo, useState } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';

import {
  BodyFormItemCreateResponseSchema,
  BodyFormItemJson,
  BodyFormItemListItem,
  BodyFormItemListItemSchema,
  BodyFormItemListResponseSchema,
  BodyFormItemSchema,
  BodyFormItemUpdateRequestSchema,
  BodyKind,
  BodyUrlEncodedItemCreateResponseSchema,
  BodyUrlEncodedItemJson,
  BodyUrlEncodedItemListItem,
  BodyUrlEncodedItemListItemSchema,
  BodyUrlEncodedItemListResponseSchema,
  BodyUrlEncodedItemSchema,
  BodyUrlEncodedItemUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import {
  bodyFormItemCreate,
  bodyFormItemDelete,
  bodyFormItemList,
  bodyFormItemUpdate,
  bodyRawGet,
  bodyRawUpdate,
  bodyUrlEncodedItemCreate,
  bodyUrlEncodedItemDelete,
  bodyUrlEncodedItemList,
  bodyUrlEncodedItemUpdate,
} from '@the-dev-tools/spec/collection/item/body/v1/body-RequestService_connectquery';
import {
  exampleGet,
  exampleUpdate,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan/body',
)({ component: Tab });

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');
const endpointRoute = getRouteApi(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
);

function Tab() {
  const queryClient = useQueryClient();

  const { exampleId } = endpointRoute.useLoaderData();

  const query = useConnectQuery(exampleGet, { exampleId });
  const updateMutation = useConnectMutation(exampleUpdate);

  if (!query.isSuccess) return null;
  const { bodyKind } = query.data;

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        orientation='horizontal'
        value={bodyKind.toString()}
        onChange={async (key) => {
          const bodyKind = parseInt(key);
          await updateMutation.mutateAsync({ exampleId, bodyKind });

          await queryClient.setQueryData(
            createConnectQueryKey({
              schema: exampleGet,
              cardinality: 'finite',
              input: { exampleId },
            }),
            createProtobufSafeUpdater(exampleGet, (old) => {
              if (old === undefined) return undefined;
              return { ...old, bodyKind };
            }),
          );
        }}
      >
        <Radio value={BodyKind.UNSPECIFIED.toString()}>none</Radio>
        <Radio value={BodyKind.FORM_ARRAY.toString()}>form-data</Radio>
        <Radio value={BodyKind.URL_ENCODED_ARRAY.toString()}>x-www-form-urlencoded</Radio>
        <Radio value={BodyKind.RAW.toString()}>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(bodyKind),
        Match.when(BodyKind.FORM_ARRAY, () => <FormDataTableLoader />),
        Match.when(BodyKind.URL_ENCODED_ARRAY, () => <UrlEncodedTableLoader />),
        Match.when(BodyKind.RAW, () => <RawFormLoader />),
        Match.orElse(() => null),
      )}
    </div>
  );
}

const FormDataTableLoader = () => {
  const { exampleId } = endpointRoute.useLoaderData();
  const query = useConnectQuery(bodyFormItemList, { exampleId });
  if (!query.isSuccess) return null;
  return <FormDataTable items={query.data.items} />;
};

interface FormDataTableProps {
  items: BodyFormItemListItem[];
}

const FormDataTable = ({ items }: FormDataTableProps) => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();
  const { exampleId } = endpointRoute.useLoaderData();

  const createMutation = useConnectMutation(bodyFormItemCreate);
  const updateMutation = useConnectMutation(bodyFormItemUpdate);
  const { mutate: deleteMutate } = useConnectMutation(bodyFormItemDelete);

  const makeItem = useCallback(
    (bodyId?: string, item?: BodyFormItemJson) => ({
      ...item,
      bodyId: bodyId ?? '',
      enabled: true,
    }),
    [],
  );
  const values = useMemo(
    () => ({
      items: [...items.map((_): BodyFormItemJson => toJson(BodyFormItemListItemSchema, _)), makeItem()],
    }),
    [items, makeItem],
  );
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({
    control: form.control,
    name: 'items',
    keyName: 'bodyId',
  });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<BodyFormItemJson>();
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
                const bodyIdJson = getValues(`items.${row.index}.bodyId`);
                if (bodyIdJson === undefined) return;
                const { bodyId } = fromJson(BodyFormItemSchema, {
                  bodyId: bodyIdJson,
                });
                deleteMutate({ bodyId });
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
    getRowId: (_) => _.bodyId ?? '',
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(() => {
    const items = pipe(
      getValues('items'),
      Array.dropRight(1),
      Array.map((_) => fromJson(BodyFormItemListItemSchema, _)),
    );
    queryClient.setQueryData(
      createConnectQueryKey({
        schema: bodyFormItemList,
        cardinality: 'finite',
        input: { items },
      }),
      createProtobufSafeUpdater(bodyFormItemList, () => create(BodyFormItemListResponseSchema, { items })),
    );
  }, [getValues, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    getRowId: (_) => _.bodyId,
    onCreate: async (body) => {
      const response = await createMutation.mutateAsync({ ...body, exampleId });
      return toJson(BodyFormItemCreateResponseSchema, response).bodyId ?? '';
    },
    onUpdate: (body) => updateMutation.mutateAsync(fromJson(BodyFormItemUpdateRequestSchema, body)),
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

const UrlEncodedTableLoader = () => {
  const { exampleId } = endpointRoute.useLoaderData();
  const query = useConnectQuery(bodyUrlEncodedItemList, { exampleId });
  if (!query.isSuccess) return null;
  return <UrlEncodedTable items={query.data.items} />;
};

interface UrlEncodedTableProps {
  items: BodyUrlEncodedItemListItem[];
}

const UrlEncodedTable = ({ items }: UrlEncodedTableProps) => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();
  const { exampleId } = endpointRoute.useLoaderData();

  const createMutation = useConnectMutation(bodyUrlEncodedItemCreate);
  const updateMutation = useConnectMutation(bodyUrlEncodedItemUpdate);
  const { mutate: deleteMutate } = useConnectMutation(bodyUrlEncodedItemDelete);

  const makeItem = useCallback(
    (bodyId?: string, item?: BodyUrlEncodedItemJson) => ({
      ...item,
      bodyId: bodyId ?? '',
      enabled: true,
    }),
    [],
  );
  const values = useMemo(
    () => ({
      items: [...items.map((_): BodyUrlEncodedItemJson => toJson(BodyUrlEncodedItemListItemSchema, _)), makeItem()],
    }),
    [items, makeItem],
  );
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({
    control: form.control,
    name: 'items',
    keyName: 'bodyId',
  });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<BodyUrlEncodedItemJson>();
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
                const bodyIdJson = getValues(`items.${row.index}.bodyId`);
                if (bodyIdJson === undefined) return;
                const { bodyId } = fromJson(BodyUrlEncodedItemSchema, {
                  bodyId: bodyIdJson,
                });
                deleteMutate({ bodyId });
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
    getRowId: (_) => _.bodyId ?? '',
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(() => {
    const items = pipe(
      getValues('items'),
      Array.dropRight(1),
      Array.map((_) => fromJson(BodyUrlEncodedItemListItemSchema, _)),
    );
    queryClient.setQueryData(
      createConnectQueryKey({
        schema: bodyUrlEncodedItemList,
        cardinality: 'finite',
        input: { items },
      }),
      createProtobufSafeUpdater(bodyUrlEncodedItemList, () => create(BodyUrlEncodedItemListResponseSchema, { items })),
    );
  }, [getValues, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    getRowId: (_) => _.bodyId,
    onCreate: async (body) => {
      const response = await createMutation.mutateAsync({ ...body, exampleId });
      return toJson(BodyUrlEncodedItemCreateResponseSchema, response).bodyId ?? '';
    },
    onUpdate: (body) => updateMutation.mutateAsync(fromJson(BodyUrlEncodedItemUpdateRequestSchema, body)),
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

const languages = ['text', 'json', 'html', 'xml'] as const;

const RawFormLoader = () => {
  const { exampleId } = endpointRoute.useLoaderData();
  const query = useConnectQuery(bodyRawGet, { exampleId });
  if (!query.isSuccess) return null;
  const body = new TextDecoder().decode(query.data.data);
  return <RawForm body={body} />;
};

interface RawFormProps {
  body: string;
}

const RawForm = ({ body }: RawFormProps) => {
  const { exampleId } = endpointRoute.useLoaderData();

  const updateMutation = useConnectMutation(bodyRawUpdate);

  const [value, setValue] = useState(body);
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
        onBlur={() =>
          void updateMutation.mutate({
            exampleId,
            data: new TextEncoder().encode(value),
          })
        }
        height='100%'
        className='col-span-full self-stretch'
        extensions={extensions}
      />
    </>
  );
};
