import { useRouteContext } from '@tanstack/react-router';
import { Struct } from 'effect';
import { useEffect } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { useDebouncedCallback } from 'use-debounce';

import {
  AssertCreateEndpoint,
  AssertDeleteEndpoint,
  AssertListEndpoint,
  AssertUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useQuery } from '~data-client';

import { ConditionField } from './condition';

interface AssertionViewProps {
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const AssertionView = ({ exampleId, isReadOnly }: AssertionViewProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { items } = useQuery(AssertListEndpoint, { exampleId });

  const form = useForm({ values: { items } });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const assertUpdateCallback = useDebouncedCallback(
    form.handleSubmit(async ({ items }) => {
      const updates = items.map((_) => dataClient.fetch(AssertUpdateEndpoint, { ...Struct.omit(_, '$typeName') }));
      await Promise.allSettled(updates);
    }),
    500,
  );

  useEffect(() => {
    const subscription = form.watch((_, { name }) => {
      if (!name) return;
      void assertUpdateCallback();
    });
    return () => void subscription.unsubscribe();
  }, [assertUpdateCallback, form]);

  return (
    <div className={tw`flex flex-col gap-2`}>
      {fieldArray.fields.map((item, index) => (
        <div className={tw`flex items-center gap-2`} key={item.id}>
          <ConditionField
            className={tw`flex-1`}
            control={form.control}
            isReadOnly={isReadOnly}
            path={`items.${index}.condition`}
          />
          <Button
            className={tw`h-8 text-red-700`}
            onPress={() => void dataClient.fetch(AssertDeleteEndpoint, { assertId: item.assertId })}
            variant='secondary'
          >
            <LuTrash2 />
          </Button>
        </div>
      ))}

      {!isReadOnly && (
        <Button onPress={() => void dataClient.fetch(AssertCreateEndpoint, { exampleId })}>New Assertion</Button>
      )}
    </div>
  );
};
