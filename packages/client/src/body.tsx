import { create } from '@bufbuild/protobuf';
import { createConnectQueryKey, createProtobufSafeUpdater, createQueryOptions } from '@connectrpc/connect-query';
import { useQueryClient, useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe } from 'effect';
import { useState } from 'react';

import {
  BodyFormItemListItem,
  BodyKind,
  BodyUrlEncodedItemListItem,
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
} from '@the-dev-tools/spec/collection/item/body/v1/body-BodyService_connectquery';
import { ExampleGetResponseSchema } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleGet,
  exampleUpdate,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectMutation, useConnectSuspenseQuery } from '~/api/connect-query';
import {
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~code-mirror/extensions';

import {
  ColumnActionDelete,
  columnActions,
  ColumnActionUndoDelta,
  columnCheckboxField,
  columnTextField,
  columnTextFieldWithReference,
  deltaFormTable,
  displayTable,
  makeDeltaItems,
  ReactTableNoMemo,
  useFormTable,
} from './form-table';

interface BodyViewProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const BodyView = ({ deltaExampleId, exampleId, isReadOnly }: BodyViewProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const queryClient = useQueryClient();

  const {
    data: { bodyKind },
  } = useConnectSuspenseQuery(exampleGet, { exampleId });
  const updateMutation = useConnectMutation(exampleUpdate);

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        isReadOnly={isReadOnly ?? false}
        onChange={async (key) => {
          await updateMutation.mutateAsync({ bodyKind: parseInt(key), exampleId });

          // TODO: remove manual update once optional field normalization is fixed
          queryClient.setQueryData(
            createConnectQueryKey({
              cardinality: 'finite',
              input: { exampleId },
              schema: exampleGet,
              transport,
            }),
            createProtobufSafeUpdater(exampleGet, (_) => ({
              ...(_ ?? create(ExampleGetResponseSchema)),
              bodyKind: parseInt(key),
            })),
          );
        }}
        orientation='horizontal'
        value={bodyKind.toString()}
      >
        <Radio value={BodyKind.UNSPECIFIED.toString()}>none</Radio>
        <Radio value={BodyKind.FORM_ARRAY.toString()}>form-data</Radio>
        <Radio value={BodyKind.URL_ENCODED_ARRAY.toString()}>x-www-form-urlencoded</Radio>
        <Radio value={BodyKind.RAW.toString()}>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(bodyKind),
        Match.when(BodyKind.FORM_ARRAY, () => {
          if (isReadOnly) return <FormDisplayTable exampleId={exampleId} />;
          if (deltaExampleId) return <FormDeltaDataTable deltaExampleId={deltaExampleId} exampleId={exampleId} />;
          return <FormDataTable exampleId={exampleId} />;
        }),
        Match.when(BodyKind.URL_ENCODED_ARRAY, () => {
          if (isReadOnly) return <UrlEncodedDisplayTable exampleId={exampleId} />;
          if (deltaExampleId) return <UrlEncodedDeltaFormTable deltaExampleId={deltaExampleId} exampleId={exampleId} />;
          return <UrlEncodedFormTable exampleId={exampleId} />;
        }),
        Match.when(BodyKind.RAW, () => <RawForm exampleId={exampleId} isReadOnly={isReadOnly} />),
        Match.orElse(() => null),
      )}
    </div>
  );
};

const formDataColumns = [
  columnCheckboxField<BodyFormItemListItem>('enabled', { meta: { divider: false } }),
  columnTextFieldWithReference<BodyFormItemListItem>('key'),
  columnTextFieldWithReference<BodyFormItemListItem>('value'),
  columnTextField<BodyFormItemListItem>('description', { meta: { divider: false } }),
];

interface FormDisplayTableProps {
  exampleId: Uint8Array;
}

const FormDisplayTable = ({ exampleId }: FormDisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyFormItemList, { exampleId });

  const table = useReactTable({
    columns: formDataColumns,
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable {...displayTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDataTableProps {
  exampleId: Uint8Array;
}

const FormDataTable = ({ exampleId }: FormDataTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyFormItemList, { exampleId });

  const { mutateAsync: create } = useConnectMutation(bodyFormItemCreate);
  const { mutateAsync: update } = useConnectMutation(bodyFormItemUpdate);

  const table = useReactTable({
    columns: [
      ...formDataColumns,
      columnActions<BodyFormItemListItem>({
        cell: ({ row }) => <ColumnActionDelete input={{ bodyId: row.original.bodyId }} schema={bodyFormItemDelete} />,
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New form data item',
    items,
    onCreate: () => create({ enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDeltaDataTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const FormDeltaDataTable = ({ deltaExampleId, exampleId }: FormDeltaDataTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const { mutateAsync: create } = useConnectMutation(bodyFormItemCreate);
  const { mutateAsync: update } = useConnectMutation(bodyFormItemUpdate);

  const [
    {
      data: { items: itemsBase },
    },
    {
      data: { items: itemsDelta },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(bodyFormItemList, { exampleId }, { transport }),
      createQueryOptions(bodyFormItemList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const items = makeDeltaItems({
    getId: (_) => _.bodyId.toString(),
    getParentId: (_) => _.parentBodyId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = deltaFormTable<BodyFormItemListItem>({
    getParentId: (_) => _.parentBodyId?.toString(),
    onCreate: ({ $typeName: _, bodyId, ...item }) =>
      create({ ...item, exampleId: deltaExampleId, parentBodyId: bodyId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...formDataColumns,
        columnActions<BodyFormItemListItem>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentBodyId !== undefined}
              input={{ bodyId: row.original.bodyId }}
              schema={bodyFormItemDelete}
            />
          ),
        }),
      ]}
      data={items}
      getCoreRowModel={getCoreRowModel()}
      getRowId={(_) => (_.parentBodyId ?? _.bodyId).toString()}
    >
      {(table) => <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />}
    </ReactTableNoMemo>
  );
};

const urlEncodedDataColumns = [
  columnCheckboxField<BodyUrlEncodedItemListItem>('enabled', { meta: { divider: false } }),
  columnTextFieldWithReference<BodyUrlEncodedItemListItem>('key'),
  columnTextFieldWithReference<BodyUrlEncodedItemListItem>('value'),
  columnTextField<BodyUrlEncodedItemListItem>('description', { meta: { divider: false } }),
];

interface UrlEncodedDisplayTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedDisplayTable = ({ exampleId }: UrlEncodedDisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyUrlEncodedItemList, { exampleId });

  const table = useReactTable({
    columns: urlEncodedDataColumns,
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable {...displayTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedFormTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedFormTable = ({ exampleId }: UrlEncodedFormTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyUrlEncodedItemList, { exampleId });

  const { mutateAsync: create } = useConnectMutation(bodyUrlEncodedItemCreate);
  const { mutateAsync: update } = useConnectMutation(bodyUrlEncodedItemUpdate);

  const table = useReactTable({
    columns: [
      ...urlEncodedDataColumns,
      columnActions<BodyUrlEncodedItemListItem>({
        cell: ({ row }) => (
          <ColumnActionDelete input={{ bodyId: row.original.bodyId }} schema={bodyUrlEncodedItemDelete} />
        ),
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New URL encoded item',
    items,
    onCreate: () => create({ enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedDeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const UrlEncodedDeltaFormTable = ({ deltaExampleId, exampleId }: UrlEncodedDeltaFormTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const { mutateAsync: create } = useConnectMutation(bodyUrlEncodedItemCreate);
  const { mutateAsync: update } = useConnectMutation(bodyUrlEncodedItemUpdate);

  const [
    {
      data: { items: itemsBase },
    },
    {
      data: { items: itemsDelta },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(bodyUrlEncodedItemList, { exampleId }, { transport }),
      createQueryOptions(bodyUrlEncodedItemList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const items = makeDeltaItems({
    getId: (_) => _.bodyId.toString(),
    getParentId: (_) => _.parentBodyId?.toString(),
    itemsBase,
    itemsDelta,
  });

  const formTable = deltaFormTable<BodyUrlEncodedItemListItem>({
    getParentId: (_) => _.parentBodyId?.toString(),
    onCreate: ({ $typeName: _, bodyId, ...item }) =>
      create({ ...item, exampleId: deltaExampleId, parentBodyId: bodyId }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...urlEncodedDataColumns,
        columnActions<BodyUrlEncodedItemListItem>({
          cell: ({ row }) => (
            <ColumnActionUndoDelta
              hasDelta={row.original.parentBodyId !== undefined}
              input={{ bodyId: row.original.bodyId }}
              schema={bodyFormItemDelete}
            />
          ),
        }),
      ]}
      data={items}
      getCoreRowModel={getCoreRowModel()}
      getRowId={(_) => (_.parentBodyId ?? _.bodyId).toString()}
    >
      {(table) => <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />}
    </ReactTableNoMemo>
  );
};

interface RawFormProps {
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

const RawForm = ({ exampleId, isReadOnly }: RawFormProps) => {
  const {
    data: { data },
  } = useConnectSuspenseQuery(bodyRawGet, { exampleId });
  const body = new TextDecoder().decode(data);

  const updateMutation = useConnectMutation(bodyRawUpdate);

  const [value, setValue] = useState(body);
  const [language, setLanguage] = useState<CodeMirrorMarkupLanguage>('text');

  const extensions = useCodeMirrorLanguageExtensions(language);

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
        onSelectionChange={(_) => void setLanguage(_ as CodeMirrorMarkupLanguage)}
        selectedKey={language}
        triggerClassName={tw`px-4 py-1`}
      >
        {CodeMirrorMarkupLanguages.map((_) => (
          <ListBoxItem id={_} key={_}>
            {_}
          </ListBoxItem>
        ))}
      </Select>

      <CodeMirror
        className='col-span-full self-stretch'
        extensions={extensions}
        height='100%'
        onBlur={() => void updateMutation.mutate({ data: new TextEncoder().encode(value), exampleId })}
        onChange={setValue}
        readOnly={isReadOnly ?? false}
        value={value}
      />
    </>
  );
};
