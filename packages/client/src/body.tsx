import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe } from 'effect';
import { useContext, useState } from 'react';

import {
  BodyFormItemDeltaListItem,
  BodyFormItemListItem,
  BodyKind,
  BodyUrlEncodedItemDeltaListItem,
  BodyUrlEncodedItemListItem,
} from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import { bodyRawGet, bodyRawUpdate } from '@the-dev-tools/spec/collection/item/body/v1/body-BodyService_connectquery';
import {
  BodyFormItemCreateEndpoint,
  BodyFormItemDeleteEndpoint,
  BodyFormItemDeltaCreateEndpoint,
  BodyFormItemDeltaDeleteEndpoint,
  BodyFormItemDeltaListEndpoint,
  BodyFormItemDeltaResetEndpoint,
  BodyFormItemDeltaUpdateEndpoint,
  BodyFormItemListEndpoint,
  BodyFormItemUpdateEndpoint,
  BodyUrlEncodedItemCreateEndpoint,
  BodyUrlEncodedItemDeleteEndpoint,
  BodyUrlEncodedItemDeltaCreateEndpoint,
  BodyUrlEncodedItemDeltaDeleteEndpoint,
  BodyUrlEncodedItemDeltaListEndpoint,
  BodyUrlEncodedItemDeltaResetEndpoint,
  BodyUrlEncodedItemDeltaUpdateEndpoint,
  BodyUrlEncodedItemListEndpoint,
  BodyUrlEncodedItemUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/body/v1/body.endpoints.ts';
import {
  ExampleGetEndpoint,
  ExampleUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.ts';
import { ReferenceService } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectMutation, useConnectSuspenseQuery } from '~api/connect-query';
import { GenericMessage } from '~api/utils';
import {
  baseCodeMirrorExtensions,
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~code-mirror/extensions';
import { useReactRender } from '~react-render';

import {
  columnActionsCommon,
  columnActionsDeltaCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  displayTable,
  ReactTableNoMemo,
  useFormTable,
} from './form-table';
import { ReferenceContext } from './reference';

interface BodyViewProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const BodyView = ({ deltaExampleId, exampleId, isReadOnly }: BodyViewProps) => {
  const transport = useTransport();
  const controller = useController();

  const { bodyKind } = useSuspense(ExampleGetEndpoint, transport, { exampleId });

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        isReadOnly={isReadOnly ?? false}
        // TODO: check if the endpoint schema is correct
        onChange={(key) => controller.fetch(ExampleUpdateEndpoint, transport, { bodyKind: parseInt(key), exampleId })}
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
  columnCheckboxField<GenericMessage<BodyFormItemListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<BodyFormItemListItem>>('key'),
  columnReferenceField<GenericMessage<BodyFormItemListItem>>('value'),
  columnTextField<GenericMessage<BodyFormItemListItem>>('description', { meta: { divider: false } }),
];

interface FormDisplayTableProps {
  exampleId: Uint8Array;
}

const FormDisplayTable = ({ exampleId }: FormDisplayTableProps) => {
  const transport = useTransport();

  const { items } = useSuspense(BodyFormItemListEndpoint, transport, { exampleId });

  const table = useReactTable({
    columns: formDataColumns,
    data: items,
  });

  return <DataTable {...displayTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDataTableProps {
  exampleId: Uint8Array;
}

const FormDataTable = ({ exampleId }: FormDataTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items: GenericMessage<BodyFormItemListItem>[] = useSuspense(BodyFormItemListEndpoint, transport, {
    exampleId,
  }).items;

  const table = useReactTable({
    columns: [
      ...formDataColumns,
      columnActionsCommon<GenericMessage<BodyFormItemListItem>>({
        onDelete: (_) => controller.fetch(BodyFormItemDeleteEndpoint, transport, { bodyId: _.bodyId }),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New form data item',
    items,
    onCreate: () => controller.fetch(BodyFormItemCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(BodyFormItemUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface FormDeltaDataTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const FormDeltaDataTable = ({ deltaExampleId, exampleId }: FormDeltaDataTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items: GenericMessage<BodyFormItemDeltaListItem>[] = useSuspense(BodyFormItemDeltaListEndpoint, transport, {
    exampleId: deltaExampleId,
    originId: exampleId,
  }).items;

  const formTable = useFormTable({
    createLabel: 'New form data item',
    items,
    onCreate: () => controller.fetch(BodyFormItemDeltaCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(BodyFormItemDeltaUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...formDataColumns,
        columnActionsDeltaCommon<GenericMessage<BodyFormItemDeltaListItem>>({
          onDelete: (_) => controller.fetch(BodyFormItemDeltaDeleteEndpoint, transport, { bodyId: _.bodyId }),
          onReset: (_) => controller.fetch(BodyFormItemDeltaResetEndpoint, transport, { bodyId: _.bodyId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.bodyId.toString()}
    >
      {(table) => <DataTable {...formTable} table={table} />}
    </ReactTableNoMemo>
  );
};

const urlEncodedDataColumns = [
  columnCheckboxField<GenericMessage<BodyUrlEncodedItemListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<BodyUrlEncodedItemListItem>>('key'),
  columnReferenceField<GenericMessage<BodyUrlEncodedItemListItem>>('value'),
  columnTextField<GenericMessage<BodyUrlEncodedItemListItem>>('description', { meta: { divider: false } }),
];

interface UrlEncodedDisplayTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedDisplayTable = ({ exampleId }: UrlEncodedDisplayTableProps) => {
  const transport = useTransport();

  const { items } = useSuspense(BodyUrlEncodedItemListEndpoint, transport, { exampleId });

  const table = useReactTable({
    columns: urlEncodedDataColumns,
    data: items,
  });

  return <DataTable {...displayTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedFormTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedFormTable = ({ exampleId }: UrlEncodedFormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items: GenericMessage<BodyUrlEncodedItemListItem>[] = useSuspense(BodyUrlEncodedItemListEndpoint, transport, {
    exampleId,
  }).items;

  const table = useReactTable({
    columns: [
      ...urlEncodedDataColumns,
      columnActionsCommon<GenericMessage<BodyUrlEncodedItemListItem>>({
        onDelete: (_) => controller.fetch(BodyUrlEncodedItemDeleteEndpoint, transport, { bodyId: _.bodyId }),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New URL encoded item',
    items,
    onCreate: () => controller.fetch(BodyUrlEncodedItemCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(BodyUrlEncodedItemUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return <DataTable {...formTable} table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedDeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const UrlEncodedDeltaFormTable = ({ deltaExampleId, exampleId }: UrlEncodedDeltaFormTableProps) => {
  const transport = useTransport();
  const controller = useController();

  const items: GenericMessage<BodyUrlEncodedItemDeltaListItem>[] = useSuspense(
    BodyUrlEncodedItemDeltaListEndpoint,
    transport,
    {
      exampleId: deltaExampleId,
      originId: exampleId,
    },
  ).items;

  const formTable = useFormTable({
    createLabel: 'New URL encoded item',
    items,
    onCreate: () => controller.fetch(BodyUrlEncodedItemDeltaCreateEndpoint, transport, { enabled: true, exampleId }),
    onUpdate: ({ $typeName: _, ...item }) => controller.fetch(BodyUrlEncodedItemDeltaUpdateEndpoint, transport, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...urlEncodedDataColumns,
        columnActionsDeltaCommon<GenericMessage<BodyUrlEncodedItemDeltaListItem>>({
          onDelete: (_) => controller.fetch(BodyUrlEncodedItemDeltaDeleteEndpoint, transport, { bodyId: _.bodyId }),
          onReset: (_) => controller.fetch(BodyUrlEncodedItemDeltaResetEndpoint, transport, { bodyId: _.bodyId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.bodyId.toString()}
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
  // TODO: switch to Data Client Endpoints
  const {
    data: { data },
  } = useConnectSuspenseQuery(bodyRawGet, { exampleId });
  const body = new TextDecoder().decode(data);

  const updateMutation = useConnectMutation(bodyRawUpdate);

  const [value, setValue] = useState(body);
  const [language, setLanguage] = useState<CodeMirrorMarkupLanguage>('text');

  // Get base language extensions
  const languageExtensions = useCodeMirrorLanguageExtensions(language);

  // Get reference context and setup for variable autocompletion
  const context = useContext(ReferenceContext);
  const transport = useTransport();
  const client = createClient(ReferenceService, transport);
  const reactRender = useReactRender();

  // TODO: use pre-composed extensions instead of duplicating code here
  // Combine language extensions with reference extensions
  const combinedExtensions = [...languageExtensions, ...baseCodeMirrorExtensions({ client, context, reactRender })];

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
        extensions={combinedExtensions}
        height='100%'
        onBlur={() => void updateMutation.mutate({ data: new TextEncoder().encode(value), exampleId })}
        onChange={setValue}
        readOnly={isReadOnly ?? false}
        value={value}
      />
    </>
  );
};
