import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useRouteContext } from '@tanstack/react-router';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Match, Option, pipe, Predicate } from 'effect';
import { Ulid } from 'id128';
import { useContext, useState } from 'react';
import { useDragAndDrop } from 'react-aria-components';
import {
  BodyFormDeltaListItem,
  BodyFormListItem,
  BodyKind,
  BodyUrlEncodedDeltaListItem,
  BodyUrlEncodedListItem,
} from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import {
  BodyFormCreateEndpoint,
  BodyFormDeleteEndpoint,
  BodyFormDeltaCreateEndpoint,
  BodyFormDeltaDeleteEndpoint,
  BodyFormDeltaListEndpoint,
  BodyFormDeltaResetEndpoint,
  BodyFormDeltaUpdateEndpoint,
  BodyFormListEndpoint,
  BodyFormMoveEndpoint,
  BodyFormUpdateEndpoint,
  BodyRawGetEndpoint,
  BodyRawUpdateEndpoint,
  BodyUrlEncodedCreateEndpoint,
  BodyUrlEncodedDeleteEndpoint,
  BodyUrlEncodedDeltaCreateEndpoint,
  BodyUrlEncodedDeltaDeleteEndpoint,
  BodyUrlEncodedDeltaListEndpoint,
  BodyUrlEncodedDeltaResetEndpoint,
  BodyUrlEncodedDeltaUpdateEndpoint,
  BodyUrlEncodedListEndpoint,
  BodyUrlEncodedMoveEndpoint,
  BodyUrlEncodedUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/body/v1/body.endpoints.ts';
import {
  ExampleGetEndpoint,
  ExampleUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.ts';
import { ReferenceService } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { MovePosition } from '@the-dev-tools/spec/resources/v1/resources_pb';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { GenericMessage } from '~api/utils';
import {
  baseCodeMirrorExtensions,
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~code-mirror/extensions';
import { useQuery } from '~data-client';
import { useReactRender } from '~react-render';
import {
  columnActionsCommon,
  columnActionsDeltaCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  displayTable,
  makeDeltaItems,
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
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { bodyKind } = useQuery(ExampleGetEndpoint, { exampleId });

  return (
    <div className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <RadioGroup
        aria-label='Body type'
        className='h-7 justify-center'
        isReadOnly={isReadOnly ?? false}
        onChange={(key) => dataClient.fetch(ExampleUpdateEndpoint, { bodyKind: parseInt(key), exampleId })}
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
        Match.when(BodyKind.RAW, () => (
          <RawForm deltaExampleId={deltaExampleId} exampleId={exampleId} isReadOnly={isReadOnly} />
        )),
        Match.orElse(() => null),
      )}
    </div>
  );
};

const formDataColumns = [
  columnCheckboxField<GenericMessage<BodyFormListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<BodyFormListItem>>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<GenericMessage<BodyFormListItem>>('value', { allowFiles: true }),
  columnTextField<GenericMessage<BodyFormListItem>>('description', { meta: { divider: false } }),
];

interface FormDisplayTableProps {
  exampleId: Uint8Array;
}

const FormDisplayTable = ({ exampleId }: FormDisplayTableProps) => {
  const { items } = useQuery(BodyFormListEndpoint, { exampleId });

  const table = useReactTable({
    columns: formDataColumns,
    data: items,
  });

  return (
    <DataTable
      {...displayTable<GenericMessage<BodyFormListItem>>()}
      table={table}
      tableAria-label='Body form items'
      wrapperClassName={tw`col-span-full`}
    />
  );
};

interface FormDataTableProps {
  exampleId: Uint8Array;
}

const FormDataTable = ({ exampleId }: FormDataTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items: GenericMessage<BodyFormListItem>[] = useQuery(BodyFormListEndpoint, {
    exampleId,
  }).items;

  const table = useReactTable({
    columns: [
      ...formDataColumns,
      columnActionsCommon<GenericMessage<BodyFormListItem>>({
        onDelete: (_) => dataClient.fetch(BodyFormDeleteEndpoint, { bodyId: _.bodyId }),
      }),
    ],
    data: items,
    getRowId: (_) => Ulid.construct(_.bodyId).toCanonical(),
  });

  const formTable = useFormTable({
    createLabel: 'New form data item',
    items,
    onCreate: async () => {
      await dataClient.fetch(BodyFormCreateEndpoint, { enabled: true, exampleId });
      // TODO: improve key matching
      await dataClient.controller.expireAll({ testKey: (_) => _.startsWith(`["${BodyFormDeltaListEndpoint.name}"`) });
    },
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(BodyFormUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(BodyFormMoveEndpoint, {
          bodyId: Ulid.fromCanonical(sourceIdCan).bytes,
          exampleId,
          position,
          targetBodyId: Ulid.fromCanonical(targetIdCan).bytes,
        });
      }),
    renderDropIndicator: () => <tr className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />,
  });

  return (
    <DataTable
      {...formTable}
      table={table}
      tableAria-label='Body form items'
      tableDragAndDropHooks={dragAndDropHooks}
      wrapperClassName={tw`col-span-full`}
    />
  );
};

interface FormDeltaDataTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const FormDeltaDataTable = ({ deltaExampleId: exampleId, exampleId: originId }: FormDeltaDataTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items = pipe(useQuery(BodyFormDeltaListEndpoint, { exampleId, originId }).items, (_: BodyFormDeltaListItem[]) =>
    makeDeltaItems(_, 'bodyId'),
  );

  const formTable = useFormTable({
    createLabel: 'New form data item',
    items,
    onCreate: () => dataClient.fetch(BodyFormDeltaCreateEndpoint, { enabled: true, exampleId, originId }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(BodyFormDeltaUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...formDataColumns,
        columnActionsDeltaCommon<GenericMessage<BodyFormDeltaListItem>>({
          onDelete: (_) => dataClient.fetch(BodyFormDeltaDeleteEndpoint, { bodyId: _.bodyId }),
          onReset: (_) => dataClient.fetch(BodyFormDeltaResetEndpoint, { bodyId: _.bodyId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.bodyId.toString()}
    >
      {(table) => <DataTable {...formTable} table={table} tableAria-label='Body form items' />}
    </ReactTableNoMemo>
  );
};

const urlEncodedDataColumns = [
  columnCheckboxField<GenericMessage<BodyUrlEncodedListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<BodyUrlEncodedListItem>>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<GenericMessage<BodyUrlEncodedListItem>>('value', { allowFiles: true }),
  columnTextField<GenericMessage<BodyUrlEncodedListItem>>('description', { meta: { divider: false } }),
];

interface UrlEncodedDisplayTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedDisplayTable = ({ exampleId }: UrlEncodedDisplayTableProps) => {
  const { items } = useQuery(BodyUrlEncodedListEndpoint, { exampleId });

  const table = useReactTable({
    columns: urlEncodedDataColumns,
    data: items,
  });

  return (
    <DataTable
      {...displayTable<GenericMessage<BodyUrlEncodedListItem>>()}
      table={table}
      tableAria-label='URL encoded body form items'
      wrapperClassName={tw`col-span-full`}
    />
  );
};

interface UrlEncodedFormTableProps {
  exampleId: Uint8Array;
}

const UrlEncodedFormTable = ({ exampleId }: UrlEncodedFormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items: GenericMessage<BodyUrlEncodedListItem>[] = useQuery(BodyUrlEncodedListEndpoint, {
    exampleId,
  }).items;

  const table = useReactTable({
    columns: [
      ...urlEncodedDataColumns,
      columnActionsCommon<GenericMessage<BodyUrlEncodedListItem>>({
        onDelete: (_) => dataClient.fetch(BodyUrlEncodedDeleteEndpoint, { bodyId: _.bodyId }),
      }),
    ],
    data: items,
    getRowId: (_) => Ulid.construct(_.bodyId).toCanonical(),
  });

  const formTable = useFormTable({
    createLabel: 'New URL encoded item',
    items,
    onCreate: async () => {
      await dataClient.fetch(BodyUrlEncodedCreateEndpoint, { enabled: true, exampleId });
      // TODO: improve key matching
      await dataClient.controller.expireAll({
        testKey: (_) => _.startsWith(`["${BodyUrlEncodedDeltaListEndpoint.name}"`),
      });
    },
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(BodyUrlEncodedUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(BodyUrlEncodedMoveEndpoint, {
          bodyId: Ulid.fromCanonical(sourceIdCan).bytes,
          exampleId,
          position,
          targetBodyId: Ulid.fromCanonical(targetIdCan).bytes,
        });
      }),
    renderDropIndicator: () => <tr className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />,
  });

  return (
    <DataTable
      {...formTable}
      table={table}
      tableAria-label='URL encoded body form items'
      tableDragAndDropHooks={dragAndDropHooks}
      wrapperClassName={tw`col-span-full`}
    />
  );
};

interface UrlEncodedDeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const UrlEncodedDeltaFormTable = ({
  deltaExampleId: exampleId,
  exampleId: originId,
}: UrlEncodedDeltaFormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items = pipe(
    useQuery(BodyUrlEncodedDeltaListEndpoint, { exampleId, originId }).items,
    (_: BodyUrlEncodedDeltaListItem[]) => makeDeltaItems(_, 'bodyId'),
  );

  const formTable = useFormTable({
    createLabel: 'New URL encoded item',
    items,
    onCreate: () => dataClient.fetch(BodyUrlEncodedDeltaCreateEndpoint, { enabled: true, exampleId, originId }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(BodyUrlEncodedDeltaUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...urlEncodedDataColumns,
        columnActionsDeltaCommon<GenericMessage<BodyUrlEncodedDeltaListItem>>({
          onDelete: (_) => dataClient.fetch(BodyUrlEncodedDeltaDeleteEndpoint, { bodyId: _.bodyId }),
          onReset: (_) => dataClient.fetch(BodyUrlEncodedDeltaResetEndpoint, { bodyId: _.bodyId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.bodyId.toString()}
    >
      {(table) => (
        <DataTable
          {...formTable}
          table={table}
          tableAria-label='URL encoded body form items'
          wrapperClassName={tw`col-span-full`}
        />
      )}
    </ReactTableNoMemo>
  );
};

interface RawFormProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

const RawForm = ({ deltaExampleId, exampleId, isReadOnly }: RawFormProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });
  const transport = useTransport();

  const bodyRaw = useQuery(BodyRawGetEndpoint, { exampleId });
  const deltaBodyRaw = useQuery(BodyRawGetEndpoint, deltaExampleId ? { exampleId: deltaExampleId } : null);

  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
  const body = new TextDecoder().decode(deltaBodyRaw?.data || bodyRaw.data);

  const [value, setValue] = useState(body);
  const [language, setLanguage] = useState<CodeMirrorMarkupLanguage>('text');

  // Get base language extensions
  const languageExtensions = useCodeMirrorLanguageExtensions(language);

  // Get reference context and setup for variable autocompletion
  const context = useContext(ReferenceContext);
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
        onBlur={() =>
          void dataClient.fetch(BodyRawUpdateEndpoint, {
            data: new TextEncoder().encode(value),
            exampleId: deltaExampleId ?? exampleId,
          })
        }
        onChange={setValue}
        readOnly={isReadOnly ?? false}
        value={value}
      />
    </>
  );
};
