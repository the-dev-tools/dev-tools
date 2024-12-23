import { create, enumToJson, fromJson, toJson } from '@bufbuild/protobuf';
import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { Array, Match, pipe, Predicate, Record, Tuple } from 'effect';
import { useEffect, useMemo } from 'react';
import {
  Collection as AriaCollection,
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItem as AriaTreeItem,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
  DialogTrigger,
} from 'react-aria-components';
import { Controller, useFieldArray, useForm } from 'react-hook-form';
import { LuChevronRight } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { assertCreateSpec, assertUpdateSpec } from '@the-dev-tools/api/spec/collection/item/request';
import {
  AssertKind,
  AssertKindSchema,
  PathKey,
  PathKeyJson,
  PathKeySchema,
  PathKind,
} from '@the-dev-tools/spec/assert/v1/assert_pb';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import {
  AssertListItem,
  AssertListItemSchema,
  AssertUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { assertList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import {
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Popover } from '@the-dev-tools/ui/popover';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextAreaFieldRHF } from '@the-dev-tools/ui/text-field';

interface AssertionViewProps {
  exampleId: Uint8Array;
}

export const AssertionView = ({ exampleId }: AssertionViewProps) => {
  const exampleGetQuery = useConnectQuery(exampleGet, { exampleId });

  const responseId = exampleGetQuery.data?.lastResponseId;
  const hasResponse = responseId !== undefined;
  const input = hasResponse ? { responseId } : {};

  const responseGetQuery = useConnectQuery(responseGet, input, { enabled: hasResponse });
  const responseHeaderListQuery = useConnectQuery(responseHeaderList, input, { enabled: hasResponse });

  const assertListQuery = useConnectQuery(assertList, { exampleId });

  if (!responseGetQuery.isSuccess || !responseHeaderListQuery.isSuccess || !assertListQuery.data) return null;

  let body;
  try {
    body = new TextDecoder().decode(responseGetQuery.data.body);
    body = JSON.parse(body) as unknown;
    if (typeof body !== 'object') body = null;
  } catch {
    body = null;
  }

  const headers = pipe(
    responseHeaderListQuery.data.items,
    Array.map((_) => [_.key, _.value] as const),
    Record.fromEntries,
  );

  return <Tab exampleId={exampleId} data={{ body, headers }} items={assertListQuery.data.items} />;
};

interface TabProps {
  exampleId: Uint8Array;
  data: Record<string, unknown>;
  items: AssertListItem[];
}

const Tab = ({ exampleId, data, items }: TabProps) => {
  const form = useForm({
    values: { items: items.map((_) => toJson(AssertListItemSchema, _)) },
  });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const assertCreateMutation = useSpecMutation(assertCreateSpec);
  const assertUpdateMutation = useSpecMutation(assertUpdateSpec);

  const assertUpdateCallback = useDebouncedCallback(
    form.handleSubmit(async ({ items }) => {
      const updates = items.map((_) => {
        const request = fromJson(AssertUpdateRequestSchema, _);
        return assertUpdateMutation.mutateAsync({ ...request, exampleId });
      });
      await Promise.allSettled(updates);
    }),
    500,
  );

  useEffect(() => {
    const subscription = form.watch(() => void assertUpdateCallback());
    return () => void subscription.unsubscribe();
  }, [assertUpdateCallback, form]);

  return (
    <>
      {fieldArray.fields.map((item, index) => (
        <div key={item.id} className={tw`flex items-center gap-2`}>
          <span>Target object</span>

          <Controller
            control={form.control}
            name={`items.${index}.path`}
            defaultValue={[]}
            render={({ field }) => (
              <PathPicker data={data} selectedPath={field.value ?? []} onSelectionChange={field.onChange} />
            )}
          />

          <SelectRHF
            control={form.control}
            name={`items.${index}.type`}
            className={tw`h-full flex-1`}
            triggerClassName={tw`h-full`}
            aria-label='Comparison Method'
          >
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.EQUAL)}>is equal to</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.NOT_EQUAL)}>is not equal to</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.CONTAINS)}>contains</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.NOT_CONTAINS)}>does not contain</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.GREATER)}>is greater than</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.GREATER_OR_EQUAL)}>
              is greater or equal to
            </ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.LESS)}>is less than</ListBoxItem>
            <ListBoxItem id={enumToJson(AssertKindSchema, AssertKind.LESS_OR_EQUAL)}>is less or equal to</ListBoxItem>
          </SelectRHF>

          <TextAreaFieldRHF
            control={form.control}
            name={`items.${index}.value`}
            className={tw`h-full flex-[2]`}
            areaClassName={tw`h-full`}
          />
        </div>
      ))}

      <Button onPress={() => void assertCreateMutation.mutate({ exampleId })}>New Assertion</Button>
    </>
  );
};

interface PathPickerProps {
  data: Record<string, unknown>;
  selectedPath: PathKeyJson[];
  onSelectionChange: (path: PathKeyJson[]) => void;
}

const PathPicker = ({ data, selectedPath, onSelectionChange }: PathPickerProps) => {
  const valueDisplay = pipe(
    selectedPath.map((_, index) =>
      pipe(
        fromJson(PathKeySchema, _),
        Match.value,
        Match.when({ kind: PathKind.UNSPECIFIED }, (_) => (
          <span key={`${index} ${_.key}`} className={tw`flex-none py-1`}>
            {_.key}
          </span>
        )),
        Match.when({ kind: PathKind.INDEX }, (_) => (
          <span key={`${index} ${_.index}`} className={tw`flex-none bg-gray-300 p-1`}>
            entry {_.index}
          </span>
        )),
        Match.when({ kind: PathKind.INDEX_ANY }, () => (
          <span key={`${index} any`} className={tw`flex-none bg-gray-300 p-1`}>
            any entry
          </span>
        )),
        Match.orElseAbsurd,
      ),
    ),
    Array.intersperse('.'),
  );

  const items = pipe(
    Array.fromRecord(data),
    Array.map(([key, data]) => {
      const path = Array.make(create(PathKeySchema, { key }));
      const ids = path.map((_) => toJson(PathKeySchema, _));
      return { id: JSON.stringify(ids), data, path };
    }),
  );

  return (
    <DialogTrigger>
      <Button className={tw`h-full flex-[2] flex-wrap justify-start`}>
        {valueDisplay.length > 0 ? valueDisplay : <span className={tw`p-1`}>Select JSON path</span>}
      </Button>
      <Popover className={tw`h-full w-1/2`}>
        {({ close }) => (
          <AriaTree
            aria-label='Path Picker'
            items={items}
            className={tw`flex flex-col gap-1`}
            onAction={(id) => {
              if (typeof id !== 'string') return;
              onSelectionChange(JSON.parse(id) as PathKeyJson[]);
              close();
            }}
          >
            {({ id, data, path }) => <PathTreeItem id={id} data={data} path={path} />}
          </AriaTree>
        )}
      </Popover>
    </DialogTrigger>
  );
};

interface PathTreeItemProps {
  id: string;
  data: unknown;
  path: Array.NonEmptyArray<PathKey>;
}

const PathTreeItem = ({ id, data, path }: PathTreeItemProps) => {
  const value = useMemo(
    () =>
      pipe(
        Match.value(data),
        Match.when(Predicate.isRecord, (_) => ({
          kind: 'object' as const,
          items: pipe(Array.fromRecord(_), Array.map(Tuple.mapFirst((_) => create(PathKeySchema, { key: _ })))),
        })),
        Match.when(Predicate.isIterable, (_) => ({
          kind: 'array' as const,
          items: pipe(
            Array.fromIterable(_),
            Array.map((data, index) => [create(PathKeySchema, { kind: PathKind.INDEX, index }), data] as const),
            // Array.prepend([create(PathKeySchema, { kind: PathKind.INDEX_ANY }), null] as const), // TODO: construct 'any' object
          ),
        })),
        Match.orElse((_) => ({ kind: 'unknown' as const, value: _ })),
      ),
    [data],
  );

  const items = useMemo(
    () =>
      pipe(
        value.kind !== 'unknown' ? value.items : [],
        Array.map(([key, data]) => {
          const itemPath = Array.append(path, key);
          const ids = itemPath.map((_) => toJson(PathKeySchema, _));
          return { id: JSON.stringify(ids), data, path: itemPath };
        }),
      ),
    [path, value],
  );

  const key = Array.lastNonEmpty(path);

  const keyDisplay = pipe(
    Match.value(key),
    Match.when({ kind: PathKind.UNSPECIFIED }, (_) => JSON.stringify(_.key)),
    Match.orElse(() => undefined),
  );

  let tag: string | undefined = undefined;
  if (value.kind !== 'unknown') tag = value.kind;
  else if (key.kind !== PathKind.UNSPECIFIED) tag = 'entry';
  if (key.kind !== PathKind.UNSPECIFIED) tag = `${tag} ${key.index}`;

  const quantity = pipe(
    Match.value(value),
    Match.when({ kind: 'object' }, (_) => `${_.items.length} keys`),
    Match.when({ kind: 'array' }, (_) => `${_.items.length} entries`),
    Match.orElse(() => undefined),
  );

  const valueDisplay = pipe(
    Match.value(value),
    Match.when({ kind: 'unknown' }, (_) => JSON.stringify(_.value)),
    Match.orElse(() => undefined),
  );

  return (
    <AriaTreeItem id={id} textValue={valueDisplay ?? tag ?? ''}>
      <AriaTreeItemContent>
        {({ level, isExpanded }) => (
          <div
            className={tw`flex cursor-pointer items-center gap-2`}
            style={{ marginInlineStart: (level - 1).toString() + 'rem' }}
          >
            {items.length > 0 && (
              <Button variant='ghost' slot='chevron'>
                <LuChevronRight
                  className={twJoin(tw`transition-transform`, !isExpanded ? tw`rotate-0` : tw`rotate-90`)}
                />
              </Button>
            )}

            {keyDisplay && <span className={tw`font-mono text-red-700`}>{keyDisplay}</span>}
            {tag && <span className={tw`bg-gray-300 p-1`}>{tag}</span>}
            {quantity && <span className={tw`text-gray-700`}>{quantity}</span>}

            {valueDisplay && (
              <>
                : <span className={tw`flex-1 break-all font-mono text-blue-700`}>{valueDisplay}</span>
              </>
            )}
          </div>
        )}
      </AriaTreeItemContent>
      <AriaCollection items={items}>
        {({ id, data, path }) => <PathTreeItem id={id} data={data} path={path} />}
      </AriaCollection>
    </AriaTreeItem>
  );
};
