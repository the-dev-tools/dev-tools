import { enumToJson } from '@bufbuild/protobuf';
import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { Array, Match, pipe, Struct } from 'effect';
import { Suspense, useEffect } from 'react';
import { DialogTrigger } from 'react-aria-components';
import { Controller, useFieldArray, useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { assertCreateSpec, assertUpdateSpec } from '@the-dev-tools/api/spec/collection/item/request';
import { AssertKind, AssertKindSchema } from '@the-dev-tools/spec/assert/v1/assert_pb';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { AssertListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { assertList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import {
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { ReferenceKey, ReferenceKeyKind } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { Button } from '@the-dev-tools/ui/button';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Popover } from '@the-dev-tools/ui/popover';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextAreaFieldRHF } from '@the-dev-tools/ui/text-field';

import { ReferenceTree } from './reference';

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

  return <Tab exampleId={exampleId} items={assertListQuery.data.items} />;
};

interface TabProps {
  exampleId: Uint8Array;
  items: AssertListItem[];
}

const Tab = ({ exampleId, items }: TabProps) => {
  const form = useForm({ values: { items } });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const assertCreateMutation = useSpecMutation(assertCreateSpec);
  const assertUpdateMutation = useSpecMutation(assertUpdateSpec);

  const assertUpdateCallback = useDebouncedCallback(
    form.handleSubmit(async ({ items }) => {
      const updates = items.map((_) => assertUpdateMutation.mutateAsync({ ...Struct.omit(_, '$typeName'), exampleId }));
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
            render={({ field }) => <PathPicker selectedPath={field.value} onSelectionChange={field.onChange} />}
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
  selectedPath: ReferenceKey[];
  onSelectionChange: (path: ReferenceKey[]) => void;
}

const PathPicker = ({ selectedPath, onSelectionChange }: PathPickerProps) => {
  const valueDisplay = pipe(
    selectedPath.map((_, index) =>
      pipe(
        Match.value(_),
        Match.when({ kind: ReferenceKeyKind.KEY }, (_) => (
          <span key={`${index} ${_.key}`} className={tw`flex-none py-1`}>
            {_.key}
          </span>
        )),
        Match.when({ kind: ReferenceKeyKind.INDEX }, (_) => (
          <span key={`${index} ${_.index}`} className={tw`flex-none bg-gray-300 p-1`}>
            entry {_.index}
          </span>
        )),
        Match.when({ kind: ReferenceKeyKind.ANY }, () => (
          <span key={`${index} any`} className={tw`flex-none bg-gray-300 p-1`}>
            any entry
          </span>
        )),
        Match.orElseAbsurd,
      ),
    ),
    Array.intersperse('.'),
  );

  return (
    <DialogTrigger>
      <Button className={tw`h-full flex-[2] flex-wrap justify-start`}>
        {valueDisplay.length > 0 ? valueDisplay : <span className={tw`p-1`}>Select JSON path</span>}
      </Button>
      <Popover className={tw`h-full w-1/2`}>
        {({ close }) => (
          <Suspense fallback='Loading references...'>
            <ReferenceTree
              onSelect={(keys) => {
                onSelectionChange(keys);
                close();
              }}
            />
          </Suspense>
        )}
      </Popover>
    </DialogTrigger>
  );
};
