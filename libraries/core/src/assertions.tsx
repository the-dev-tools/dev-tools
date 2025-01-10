import { enumToJson } from '@bufbuild/protobuf';
import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { Struct } from 'effect';
import { useEffect } from 'react';
import { Controller, useFieldArray, useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { assertCreateSpec, assertUpdateSpec } from '@the-dev-tools/api/spec/collection/item/request';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { AssertListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { assertList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import {
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { ComparisonKind, ComparisonKindSchema } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { Button } from '@the-dev-tools/ui/button';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextAreaFieldRHF } from '@the-dev-tools/ui/text-field';

import { ReferenceField } from './reference';

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
            name={`items.${index}.condition.comparison.path`}
            defaultValue={[]}
            render={({ field }) => (
              <ReferenceField path={field.value} onSelect={field.onChange} buttonClassName={tw`flex-[2]`} />
            )}
          />

          <SelectRHF
            control={form.control}
            name={`items.${index}.condition.comparison.kind`}
            className={tw`h-full flex-1`}
            triggerClassName={tw`h-full`}
            aria-label='Comparison Method'
          >
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.EQUAL)}>is equal to</ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.NOT_EQUAL)}>is not equal to</ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.CONTAINS)}>contains</ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.NOT_CONTAINS)}>
              does not contain
            </ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.GREATER)}>is greater than</ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.GREATER_OR_EQUAL)}>
              is greater or equal to
            </ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.LESS)}>is less than</ListBoxItem>
            <ListBoxItem id={enumToJson(ComparisonKindSchema, ComparisonKind.LESS_OR_EQUAL)}>
              is less or equal to
            </ListBoxItem>
          </SelectRHF>

          <TextAreaFieldRHF
            control={form.control}
            name={`items.${index}.condition.comparison.value`}
            className={tw`h-full flex-[2]`}
            areaClassName={tw`h-full`}
          />
        </div>
      ))}

      <Button onPress={() => void assertCreateMutation.mutate({ exampleId })}>New Assertion</Button>
    </>
  );
};
