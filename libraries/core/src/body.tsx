import { createClient } from '@connectrpc/connect';
import { useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Match, pipe, Struct } from 'effect';
import { useMemo, useState } from 'react';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { bodyRawUpdateSpec } from '@the-dev-tools/api/spec/collection/item/body';
import { exampleUpdateSpec } from '@the-dev-tools/api/spec/collection/item/example';
import {
  BodyFormItemListItem,
  BodyFormItemListItemSchema,
  BodyKind,
  BodyUrlEncodedItemListItem,
  BodyUrlEncodedItemListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import {
  bodyFormItemList,
  bodyRawGet,
  bodyUrlEncodedItemList,
} from '@the-dev-tools/spec/collection/item/body/v1/body-RequestService_connectquery';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { makeGenericFormTableColumns, useFormTable } from './form-table';

interface BodyViewProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const BodyView = ({ endpointId, exampleId }: BodyViewProps) => {
  const query = useConnectSuspenseQuery(exampleGet, { exampleId });
  const updateMutation = useSpecMutation(exampleUpdateSpec);

  if (!query.isSuccess) return null;
  const { bodyKind } = query.data;

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        orientation='horizontal'
        value={bodyKind.toString()}
        onChange={(key) => void updateMutation.mutate({ endpointId, exampleId, bodyKind: parseInt(key) })}
      >
        <Radio value={BodyKind.UNSPECIFIED.toString()}>none</Radio>
        <Radio value={BodyKind.FORM_ARRAY.toString()}>form-data</Radio>
        <Radio value={BodyKind.URL_ENCODED_ARRAY.toString()}>x-www-form-urlencoded</Radio>
        <Radio value={BodyKind.RAW.toString()}>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(bodyKind),
        Match.when(BodyKind.FORM_ARRAY, () => <FormDataTable exampleId={exampleId} />),
        Match.when(BodyKind.URL_ENCODED_ARRAY, () => <UrlEncodedTable exampleId={exampleId} />),
        Match.when(BodyKind.RAW, () => <RawForm exampleId={exampleId} />),
        Match.orElse(() => null),
      )}
    </div>
  );
};

interface FormDataTableProps {
  exampleId: Uint8Array;
}

const FormDataTable = ({ exampleId }: FormDataTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyFormItemList, { exampleId });

  const table = useFormTable({
    items,
    schema: BodyFormItemListItemSchema,
    columns: makeGenericFormTableColumns<BodyFormItemListItem>(),
    onCreate: (_) =>
      requestService.bodyFormItemCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.bodyId),
    onUpdate: (_) => requestService.bodyFormItemUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.bodyFormItemDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

interface UrlEncodedTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedTable = ({ exampleId }: UrlEncodedTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(bodyUrlEncodedItemList, { exampleId });

  const table = useFormTable({
    items,
    schema: BodyUrlEncodedItemListItemSchema,
    columns: makeGenericFormTableColumns<BodyUrlEncodedItemListItem>(),
    onCreate: (_) =>
      requestService.bodyUrlEncodedItemCreate({ ...Struct.omit(_, '$typeName'), exampleId }).then((_) => _.bodyId),
    onUpdate: (_) => requestService.bodyUrlEncodedItemUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => requestService.bodyUrlEncodedItemDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} wrapperClassName={tw`col-span-full`} />;
};

const languages = ['text', 'json', 'html', 'xml'] as const;

interface RawFormProps {
  exampleId: Uint8Array;
}

const RawForm = ({ exampleId }: RawFormProps) => {
  const {
    data: { data },
  } = useConnectSuspenseQuery(bodyRawGet, { exampleId });
  const body = new TextDecoder().decode(data);

  const updateMutation = useSpecMutation(bodyRawUpdateSpec);

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
        triggerClassName={tw`px-4 py-1`}
        selectedKey={language}
        onSelectionChange={(_) => void setLanguage(_ as (typeof languages)[number])}
      >
        {languages.map((_) => (
          <ListBoxItem key={_} id={_}>
            {_}
          </ListBoxItem>
        ))}
      </Select>

      <CodeMirror
        value={value}
        onChange={setValue}
        onBlur={() => void updateMutation.mutate({ exampleId, data: new TextEncoder().encode(value) })}
        height='100%'
        className='col-span-full self-stretch'
        extensions={extensions}
      />
    </>
  );
};
