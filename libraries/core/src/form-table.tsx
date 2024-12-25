import { Array, pipe } from 'effect';
import { ComponentProps, ReactNode, RefObject, useEffect, useRef } from 'react';
import {
  Control,
  FieldArrayMethodProps,
  FieldPath,
  FieldPathValue,
  FieldPathValues,
  FieldValues,
  UseFormGetValues,
  UseFormWatch,
  useWatch,
  WatchObserver,
} from 'react-hook-form';
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
    append: (value: TItem | TItem[], options?: FieldArrayMethodProps) => void;
  };
  dirtyRef?: RefObject<Map<string, TItem>>;
  getRowId: (item: TItem) => string;
  makeItem?: (id?: string, item?: Partial<TItem>) => TItem;
  onCreate?: (item: TItem) => Promise<string>;
  onUpdate: (item: TItem) => Promise<unknown>;
  onChange?: () => void;
  setData?: () => void;
}

export const useFormTableSync = <TItem, TField extends string, TFieldValues extends Record<TField, TItem[]>>({
  field,
  form: { getValues, setValue, watch },
  fieldArray,
  dirtyRef: dirtyRefProp,
  getRowId,
  makeItem,
  onUpdate,
  onCreate,
  onChange,
  setData,
}: UseFormTableSyncProps<TItem, TField, TFieldValues>) => {
  const isPending = useRef(false);
  const dirtyRef = useRef(dirtyRefProp?.current ?? new Map<string, TItem>());

  const update = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isPending.current) return void update();
    isPending.current = true;

    const dirty = dirtyRef.current;
    await pipe(
      Array.fromIterable(dirty),
      Array.map(async ([updateId, item]) => {
        dirty.delete(updateId); // Un-queue update

        if (updateId) {
          const maybeId = await onUpdate(item);
          // Unqueue update that gets created immediately after
          if (typeof maybeId === 'string') dirty.delete(maybeId);
          return;
        }

        if (!onCreate || !makeItem) return;

        const index = getValues(field).length - 1;
        const id = await onCreate(item);

        setValue(`${field}.${index}`, makeItem(id, item));
        dirty.delete(id); // Delete update that gets queued by setting new id

        fieldArray.append(makeItem(), { shouldFocus: false });

        // Redirect outdated queued update to the new id
        const outdated = dirty.get('');
        if (!outdated) return;
        dirty.delete(getRowId(outdated));
        dirty.set(id, makeItem(id, outdated));
      }),
      (_) => Promise.allSettled(_),
    );

    isPending.current = false;
    onChange?.();
  }, 500);

  useEffect(() => {
    const subscription = watch((_, { name }) => {
      const rowName = name?.match(new RegExp(`(^${field}.[\\d]+)`, 'g'))?.[0] as `${TField}.${number}` | undefined;
      if (!rowName) return;
      const rowValues = getValues(rowName);
      dirtyRef.current.set(getRowId(rowValues), rowValues);
      void update();
    });
    return () => void subscription.unsubscribe();
  }, [field, getRowId, getValues, update, watch]);

  useEffect(() => () => void update.flush()?.then(() => void setData?.()), [setData, update]);
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

interface FormWatchProps<
  TFieldValues extends FieldValues = FieldValues,
  TFieldNames extends readonly FieldPath<TFieldValues>[] = readonly FieldPath<TFieldValues>[],
> {
  name: readonly [...TFieldNames];
  control: Control<TFieldValues>;
  children: (values: FieldPathValues<TFieldValues, TFieldNames>) => ReactNode;
}

export const FormWatch = <
  TFieldValues extends FieldValues = FieldValues,
  TFieldNames extends readonly FieldPath<TFieldValues>[] = readonly FieldPath<TFieldValues>[],
>({
  children,
  ...props
}: FormWatchProps<TFieldValues, TFieldNames>) => {
  const values = useWatch(props) as FieldPathValues<TFieldValues, TFieldNames>;
  return children(values);
};

type TaskType = 'change' | (string & {});

interface UseFieldArrayTasksProps<
  TFieldValues extends FieldValues,
  TItemPath extends FieldPath<TFieldValues>,
  TKey,
  TItem extends FieldPathValue<TFieldValues, TItemPath> = FieldPathValue<TFieldValues, TItemPath>,
> {
  form: {
    getValues: UseFormGetValues<TFieldValues>;
    watch: UseFormWatch<TFieldValues>;
  };
  itemPath: (index: number) => TItemPath;
  itemKey: (item: TItem) => TKey;
  onTask: (item: TItem, type: TaskType) => Promise<void>;
  wait?: number;
}

export const useFieldArrayTasks = <
  TFieldValues extends FieldValues,
  TItemPath extends FieldPath<TFieldValues>,
  TKey,
  TItem extends FieldPathValue<TFieldValues, TItemPath> = FieldPathValue<TFieldValues, TItemPath>,
>({
  form,
  itemPath,
  itemKey,
  onTask,
  wait = 200,
}: UseFieldArrayTasksProps<TFieldValues, TItemPath, TKey, TItem>) => {
  const isPending = useRef(false);
  const tasks = useRef(new Map<TKey, [TItem, TaskType]>()).current;
  const ignoreChanges = useRef(new Set<TKey>()).current;

  const processTasks = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isPending.current) return void processTasks();
    isPending.current = true;

    await pipe(
      Array.fromIterable(tasks),
      Array.map(async ([key, task]) => {
        const [item, type] = task;
        ignoreChanges.add(key);
        await onTask(item, type);
        ignoreChanges.delete(key);
        if (tasks.get(key) === task) tasks.delete(key);
      }),
      (_) => Promise.allSettled(_),
    );

    isPending.current = false;
  }, wait);

  useEffect(() => {
    const subscription = form.watch((_, { name }) => {
      const arrayPath = itemPath(0).slice(0, -1);
      const indexRegex = new RegExp(`${arrayPath}([\\d]+)\\.`, 'g');
      const indexMatch = name?.matchAll(indexRegex).next().value?.[1] as `${number}` | undefined;

      if (indexMatch === undefined) return;

      const index = parseInt(indexMatch);
      const item = form.getValues(itemPath(index));
      const key = itemKey(item);

      if (ignoreChanges.has(key)) return;

      tasks.set(key, [item, 'change']);
      void processTasks();
    });

    return () => {
      subscription.unsubscribe();
      void processTasks.flush();
    };
  }, [tasks, form, ignoreChanges, itemKey, itemPath, processTasks]);

  return { tasks, processTasks };
};
