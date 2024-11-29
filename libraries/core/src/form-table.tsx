import { Array, pipe } from 'effect';
import { ComponentProps, useEffect, useRef } from 'react';
import { FieldArrayMethodProps, WatchObserver } from 'react-hook-form';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { tw } from '@the-dev-tools/ui/tailwind-literal';

export interface UseFormTableSyncProps<TItem, TField extends string, TFieldValues extends Record<TField, TItem[]>> {
  field: TField;
  form: {
    getValues: {
      (path: TField): NoInfer<TItem>[];
      (path: `${TField}.${number}`): NoInfer<TItem>;
    };
    setValue: (path: `${TField}.${number}`, item: TItem) => void;
    watch: (callback: WatchObserver<TFieldValues>) => { unsubscribe: () => void };
  };
  fieldArray: {
    append: (value: TItem, options?: FieldArrayMethodProps) => void;
  };
  getRowId: (item: TItem) => string;
  makeItem: (id?: string, item?: Partial<TItem>) => TItem;
  onCreate: (item: TItem) => Promise<string>;
  onUpdate: (item: TItem) => Promise<unknown>;
  onChange?: () => void;
  setData?: () => void;
}

export const useFormTableSync = <TItem, TField extends string, TFieldValues extends Record<TField, TItem[]>>({
  field,
  form: { getValues, setValue, watch },
  fieldArray,
  getRowId,
  makeItem,
  onUpdate,
  onCreate,
  onChange,
  setData,
}: UseFormTableSyncProps<TItem, TField, TFieldValues>) => {
  const isUpdatingItems = useRef(false);
  const updateItemQueueMap = useRef(new Map<string, TItem>());
  const updateItems = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isUpdatingItems.current) return void updateItems();
    isUpdatingItems.current = true;

    const updates = updateItemQueueMap.current;
    await pipe(
      Array.fromIterable(updates),
      Array.map(async ([updateId, item]) => {
        updates.delete(updateId); // Un-queue update

        if (updateId) return void (await onUpdate(item));

        const index = getValues(field).length - 1;
        const id = await onCreate(item);

        setValue(`${field}.${index}`, makeItem(id, item));
        updates.delete(id); // Delete update that gets queued by setting new id

        fieldArray.append(makeItem(), { shouldFocus: false });

        // Redirect outdated queued update to the new id
        const outdated = updates.get('');
        if (!outdated) return;
        updates.delete(getRowId(outdated));
        updates.set(id, makeItem(id, outdated));
      }),
      (_) => Promise.allSettled(_),
    );

    isUpdatingItems.current = false;
    onChange?.();
  }, 500);

  useEffect(() => {
    const subscription = watch((_, { name }) => {
      const rowName = name?.match(new RegExp(`(^${field}.[\\d]+)`, 'g'))?.[0] as `${TField}.${number}` | undefined;
      if (!rowName) return;
      const rowValues = getValues(rowName);
      updateItemQueueMap.current.set(getRowId(rowValues), rowValues);
      void updateItems();
    });
    return () => void subscription.unsubscribe();
  }, [field, getRowId, getValues, updateItems, watch]);

  useEffect(() => () => void updateItems.flush()?.then(() => void setData?.()), [setData, updateItems]);
};

export interface HidePlaceholderCellProps extends ComponentProps<'div'> {
  row: { index: number };
  table: { getRowCount: () => number };
}

export const HidePlaceholderCell = ({
  className,
  row: { index },
  table: { getRowCount },
  ...props
}: HidePlaceholderCellProps) => (
  <div {...props} className={twJoin(className, index + 1 === getRowCount() && tw`invisible`)} />
);
