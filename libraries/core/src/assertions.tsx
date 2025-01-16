import { Struct } from 'effect';
import { useEffect } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation, useConnectQuery } from '@the-dev-tools/api/connect-query';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { AssertListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  assertCreate,
  assertList,
  assertUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import {
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { ConditionField } from './condition';

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

  const assertCreateMutation = useConnectMutation(assertCreate);
  const assertUpdateMutation = useConnectMutation(assertUpdate);

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
    <div className={tw`flex flex-col gap-2`}>
      {fieldArray.fields.map((item, index) => (
        <ConditionField key={item.id} control={form.control} path={`items.${index}.condition`} />
      ))}

      <Button onPress={() => void assertCreateMutation.mutate({ exampleId })}>New Assertion</Button>
    </div>
  );
};
