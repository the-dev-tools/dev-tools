import { eq, useLiveQuery } from '@tanstack/react-db';
import { pipe } from 'effect';
import { Ulid } from 'id128';
import { useEffect } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { useDebouncedCallback } from 'use-debounce';
import { HttpAssertCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { messageData } from '~/api-new/protobuf';
import { ReferenceFieldRHF } from '~/reference';

export interface AssertPanelProps {
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const AssertPanel = ({ httpId, isReadOnly = false }: AssertPanelProps) => {
  const collection = useApiCollection(HttpAssertCollectionSchema);

  const { data: items } = useLiveQuery((_) => _.from({ item: collection }).where((_) => eq(_.item.httpId, httpId)));

  const form = useForm({
    resetOptions: { keepDirtyValues: true },
    values: { items },
  });

  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const assertUpdateCallback = useDebouncedCallback(
    form.handleSubmit(
      ({ items }) =>
        void pipe(
          items.map((_) => messageData(_)),
          (_) => collection.utils.update(_),
        ),
    ),
    500,
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/incompatible-library
    const subscription = form.watch((_, { name }) => {
      if (!name) return;
      void assertUpdateCallback();
    });
    return () => void subscription.unsubscribe();
  }, [assertUpdateCallback, form]);

  const canAdd = useWatch({
    compute: ({ items }) => {
      if (items.length < 1) return true;
      return !!items[items.length - 1]?.value;
    },
    control: form.control,
  });

  return (
    <div className={tw`flex flex-col gap-2`}>
      {fieldArray.fields.map((item, index) => (
        <div className={tw`flex items-center gap-2`} key={item.id}>
          <ReferenceFieldRHF
            allowFiles
            className={tw`flex-1`}
            control={form.control}
            name={`items.${index}.value`}
            placeholder='Enter value to compare'
            readOnly={isReadOnly}
          />
          <Button
            className={tw`h-8 text-red-700`}
            onPress={() => void collection.utils.delete({ httpAssertId: item.httpAssertId })}
            variant='secondary'
          >
            <LuTrash2 />
          </Button>
        </div>
      ))}

      {!isReadOnly && (
        <Button
          isDisabled={!canAdd}
          onPress={() => void collection.utils.insert({ httpAssertId: Ulid.generate().bytes, httpId })}
        >
          New Assertion
        </Button>
      )}
    </div>
  );
};
