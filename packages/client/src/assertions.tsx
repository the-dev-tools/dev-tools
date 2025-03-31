import { Struct } from 'effect';
import { useEffect } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';

import {
  assertCreate,
  assertList,
  assertUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectMutation, useConnectSuspenseQuery } from '~/api/connect-query';

import { ConditionField } from './condition';

interface AssertionViewProps {
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const AssertionView = ({ exampleId, isReadOnly }: AssertionViewProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(assertList, { exampleId });

  const form = useForm({ values: { items } });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const assertCreateMutation = useConnectMutation(assertCreate);
  const assertUpdateMutation = useConnectMutation(assertUpdate);

  const assertUpdateCallback = useDebouncedCallback(
    form.handleSubmit(async ({ items }) => {
      const updates = items.map((_) => assertUpdateMutation.mutateAsync({ ...Struct.omit(_, '$typeName') }));
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
        <ConditionField
          control={form.control}
          isReadOnly={isReadOnly}
          key={item.id}
          path={`items.${index}.condition`}
        />
      ))}

      {!isReadOnly && <Button onPress={() => void assertCreateMutation.mutate({ exampleId })}>New Assertion</Button>}
    </div>
  );
};
