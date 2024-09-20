import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, pipe } from 'effect';
import { useEffect, useMemo, useRef } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { useDebouncedCallback } from 'use-debounce';

import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

interface Item {
  id: string;
  enabled: boolean;
  key: string;
  value: string;
  description: string;
}

interface Props<TItem extends Item> {
  items: TItem[];
  makeItem: (item?: Partial<TItem>) => TItem;
  onCreate: (props: { item: TItem }) => Promise<{ id: string }>;
  onUpdate: (props: { item: TItem }) => Promise<unknown>;
  onDelete: (props: { id: string }) => void;
  onChange?: () => void;
}

export const useFormTable = <TItem extends Item>({
  items,
  makeItem,
  onDelete,
  onUpdate,
  onCreate,
  onChange,
}: Props<TItem>) => {
  const values = useMemo(() => ({ items: [...items, makeItem()] }), [items, makeItem]);

  const { getValues, ...form } = useForm<{ items: Item[] }>({ values });
  const { fields, remove: removeField, ...fieldArray } = useFieldArray({ name: 'items', control: form.control });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<Item>();
    return [
      accessor('enabled', {
        header: '',
        minSize: 0,
        size: 0,
        cell: ({ row, table }) => {
          if (row.index + 1 === table.getRowCount()) return null;
          return (
            <CheckboxRHF key={row.id} control={form.control} name={`items.${row.index}.enabled`} className='p-1' />
          );
        },
      }),
      accessor('key', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`items.${row.index}.key`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('value', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`items.${row.index}.value`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF
            key={row.id}
            control={form.control}
            name={`items.${row.index}.description`}
            inputClassName={tw`rounded-none border-transparent`}
          />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        minSize: 0,
        size: 0,
        cell: ({ row, table }) => {
          if (row.index + 1 === table.getRowCount()) return null;

          return (
            <Button
              className='text-red-700'
              kind='placeholder'
              variant='placeholder ghost'
              onPress={() => {
                const id = getValues(`items.${row.index}.id`);
                onDelete({ id });
                removeField(row.index);
                onChange?.();
              }}
            >
              <LuTrash2 />
            </Button>
          );
        },
      }),
    ];
  }, [form.control, getValues, onChange, onDelete, removeField]);

  const table = useReactTable({
    columns,
    data: fields,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.id,
  });

  const isUpdatingItems = useRef(false);
  const updateItemQueueMap = useRef(new Map<string, TItem>());
  const updateItems = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isUpdatingItems.current) return void updateItems();
    isUpdatingItems.current = true;

    const updates = updateItemQueueMap.current;
    await pipe(
      Array.fromIterable(updates),
      Array.map(async ([id, item]) => {
        updates.delete(id); // Un-queue update
        if (id) {
          await onUpdate({ item });
        } else {
          const { id } = await onCreate({ item });
          const index = getValues('items').length - 1;

          form.setValue(`items.${index}`, makeItem({ ...item, id }));
          updates.delete(id); // Delete update that gets queued by setting new id

          fieldArray.append(makeItem(), { shouldFocus: false });

          // Redirect outdated queued update to the new id
          const outdated = updates.get('');
          if (outdated !== undefined) {
            updates.delete('');
            updates.set(id, makeItem({ ...outdated, id }));
          }
        }
      }),
      (_) => Promise.allSettled(_),
    );

    isUpdatingItems.current = false;
    onChange?.();
  }, 500);

  useEffect(() => {
    const watch = form.watch((_, { name }) => {
      const rowName = name?.match(/(^items.[\d]+)/g)?.[0] as `items.${number}` | undefined;
      if (!rowName) return;
      const rowValues = getValues(rowName);
      updateItemQueueMap.current.set(rowValues.id, rowValues as TItem);
      void updateItems();
    });
    return () => void watch.unsubscribe();
  }, [form, getValues, updateItems]);

  useEffect(() => () => void updateItems.flush(), [updateItems]);

  return table;
};
