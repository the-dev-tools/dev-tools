import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe, Struct } from 'effect';
import { useMemo, useState } from 'react';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import {
  BodyFormItemListItem,
  BodyFormItemListItemSchema,
  BodyKind,
  BodyService,
  BodyUrlEncodedItemListItem,
  BodyUrlEncodedItemListItemSchema,
} from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import {
  bodyFormItemList,
  bodyRawGet,
  bodyRawUpdate,
  bodyUrlEncodedItemList,
} from '@the-dev-tools/spec/collection/item/body/v1/body-BodyService_connectquery';
import {
  exampleGet,
  exampleUpdate,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { CodeMirrorMarkupLanguage, CodeMirrorMarkupLanguages, useCodeMirrorExtensions } from './code-mirror';
import {
  makeGenericDeltaFormTableColumns,
  makeGenericDisplayTableColumns,
  makeGenericFormTableColumns,
  useDeltaFormTable,
  useFormTable,
} from './form-table';

interface BodyViewProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const BodyView = ({ deltaExampleId, exampleId, isReadOnly }: BodyViewProps) => {
  const query = useConnectSuspenseQuery(exampleGet, { exampleId });
  const updateMutation = useConnectMutation(exampleUpdate);

  if (!query.isSuccess) return null;
  const { bodyKind } = query.data;

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        isReadOnly={isReadOnly ?? false}
        onChange={(key) => void updateMutation.mutate({ bodyKind: parseInt(key), exampleId })}
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

interface FormDisplayTableProps {
  exampleId: Uint8Array;
}

const FormDisplayTable = ({ exampleId }: FormDisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyFormItemList, { exampleId });

  const table = useReactTable({
    columns: makeGenericDisplayTableColumns<BodyFormItemListItem>(),
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDataTableProps {
  exampleId: Uint8Array;
}

const FormDataTable = ({ exampleId }: FormDataTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(BodyService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyFormItemList, { exampleId });

  const table = useFormTable({
    columns: makeGenericFormTableColumns<BodyFormItemListItem>(),
    items,
    onCreate: (_) =>
      requestService.bodyFormItemCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.bodyId),
    onDelete: (_) => requestService.bodyFormItemDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.bodyFormItemUpdate(Struct.omit(_, '$typeName')),
    schema: BodyFormItemListItemSchema,
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDeltaDataTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const FormDeltaDataTable = ({ deltaExampleId, exampleId }: FormDeltaDataTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(BodyService, transport), [transport]);

  const [
    {
      data: { items },
    },
    {
      data: { items: deltaItems },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(bodyFormItemList, { exampleId }, { transport }),
      createQueryOptions(bodyFormItemList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const table = useDeltaFormTable({
    columns: makeGenericDeltaFormTableColumns<BodyFormItemListItem>(),
    deltaItems,
    getParentId: (_) => _.parentBodyId!,
    items,
    onCreate: (_) =>
      requestService
        .bodyFormItemCreate({
          ...Struct.omit(_, '$typeName'),
          exampleId: deltaExampleId,
          parentBodyId: _.bodyId,
        })
        .then((_) => _.bodyId),
    onDelete: (_) => requestService.bodyFormItemDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.bodyFormItemUpdate(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedDisplayTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedDisplayTable = ({ exampleId }: UrlEncodedDisplayTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyUrlEncodedItemList, { exampleId });

  const table = useReactTable({
    columns: makeGenericDisplayTableColumns<BodyUrlEncodedItemListItem>(),
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedFormTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedFormTable = ({ exampleId }: UrlEncodedFormTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(BodyService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyUrlEncodedItemList, { exampleId });

  const table = useFormTable({
    columns: makeGenericFormTableColumns<BodyUrlEncodedItemListItem>(),
    items,
    onCreate: (_) =>
      requestService.bodyUrlEncodedItemCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.bodyId),
    onDelete: (_) => requestService.bodyUrlEncodedItemDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.bodyUrlEncodedItemUpdate(Struct.omit(_, '$typeName')),
    schema: BodyUrlEncodedItemListItemSchema,
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedDeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const UrlEncodedDeltaFormTable = ({ deltaExampleId, exampleId }: UrlEncodedDeltaFormTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(BodyService, transport), [transport]);

  const [
    {
      data: { items },
    },
    {
      data: { items: deltaItems },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(bodyUrlEncodedItemList, { exampleId }, { transport }),
      createQueryOptions(bodyUrlEncodedItemList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const table = useDeltaFormTable({
    columns: makeGenericDeltaFormTableColumns<BodyUrlEncodedItemListItem>(),
    deltaItems,
    getParentId: (_) => _.parentBodyId!,
    items,
    onCreate: (_) =>
      requestService
        .bodyUrlEncodedItemCreate({
          ...Struct.omit(_, '$typeName'),
          exampleId: deltaExampleId,
          parentBodyId: _.bodyId,
        })
        .then((_) => _.bodyId),
    onDelete: (_) => requestService.bodyUrlEncodedItemDelete(Struct.omit(_, '$typeName')),
    onUpdate: (_) => requestService.bodyUrlEncodedItemUpdate(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
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

  const extensions = useCodeMirrorExtensions(language);

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
