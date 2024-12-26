import { Message } from '@bufbuild/protobuf';
import { getRouteApi } from '@tanstack/react-router';
import { AccessorKeyColumnDef, createColumnHelper, RowData } from '@tanstack/table-core';
import { Array, Option, pipe } from 'effect';
import { idEqual, Ulid } from 'id128';
import { ComponentProps, ReactNode, RefObject, useCallback, useEffect, useRef } from 'react';
import {
  Control,
  FieldArrayMethodProps,
  FieldPath,
  FieldPathValue,
  FieldPathValues,
  FieldValues,
  Path,
  UseFormGetValues,
  UseFormSetValue,
  UseFormWatch,
  useWatch,
  WatchObserver,
} from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { getMessageId } from '@the-dev-tools/api/meta';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { RHFDevTools } from './dev-tools';
import { TextFieldWithVariables } from './variable';

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

export interface Task<TItem> {
  index: number;
  item: TItem;
  type: TaskType;
}

interface UseFieldArrayTasksProps<
  TFieldValues extends FieldValues,
  TItemPath extends FieldPath<TFieldValues>,
  TKey,
  TItem extends FieldPathValue<TFieldValues, TItemPath> = FieldPathValue<TFieldValues, TItemPath>,
> {
  form: {
    getValues: UseFormGetValues<TFieldValues>;
    setValue: UseFormSetValue<TFieldValues>;
    watch: UseFormWatch<TFieldValues>;
  };
  itemPath: (index: number) => TItemPath;
  itemKey: (item: TItem) => TKey;
  onTask: (task: Task<TItem>) => Promise<void>;
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
  const tasks = useRef(new Map<TKey, Task<TItem>>()).current;
  const ignoreChanges = useRef(new Set<TKey>()).current;

  const itemTransaction = useCallback(
    (key: TKey, callback: () => void) => {
      ignoreChanges.add(key);
      callback();
      ignoreChanges.delete(key);
    },
    [ignoreChanges],
  );

  const processTasks = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isPending.current) return void processTasks();
    isPending.current = true;

    await pipe(
      Array.fromIterable(tasks),
      Array.map(async ([key, task]) => {
        await onTask(task);
        const nextTask = tasks.get(key);
        if (!nextTask) return;
        if (nextTask.index === task.index && nextTask.type === task.type) tasks.delete(key);
      }),
      (_) => Promise.allSettled(_),
    );

    isPending.current = false;
  }, wait);

  const queueTask = useCallback(
    (index: number, type: TaskType) => {
      const item = form.getValues(itemPath(index));
      const key = itemKey(item);

      if (ignoreChanges.has(key)) return;

      tasks.set(key, { index, item, type });
      void processTasks();
    },
    [form, ignoreChanges, itemKey, itemPath, processTasks, tasks],
  );

  useEffect(() => {
    const subscription = form.watch((_, { name }) => {
      const arrayPath = itemPath(0).slice(0, -2) as Path<TFieldValues>;
      const indexRegex = new RegExp(`${arrayPath}\\.([\\d]+)`, 'g');
      const indexMatch = name?.matchAll(indexRegex).next().value?.[1] as `${number}` | undefined;

      if (indexMatch === undefined) return;

      const index = parseInt(indexMatch);
      queueTask(index, 'change');
    });

    return () => {
      subscription.unsubscribe();
      void processTasks.flush();
    };
  }, [tasks, form, itemKey, itemPath, processTasks, ignoreChanges, queueTask]);

  return { queueTask, itemTransaction };
};

export interface TableFormData<T> {
  items: T[];
}

export interface TableFormItem<T> {
  data: T;
}

export interface DeltaTableFormItem<T> extends TableFormItem<T> {
  parentData: T;
}

declare module '@tanstack/table-core' {
  interface TableMeta<TData extends RowData> {
    // Form table column dependencies must be stable to avoid full table re-renders
    // which cause focus loss. Unstable dependencies must be passed via table meta
    queueTask?: (index: number, type: TaskType) => void;
    control?: Control<TableFormData<TData>>;
  }
}

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface GenericFormTableItem extends Message {
  enabled: boolean;
  key: string;
  value: string;
  description: string;
}

const genericFormTableColumnHelper = createColumnHelper<TableFormItem<GenericFormTableItem>>();

const genericFormTableColumnsShared = [
  genericFormTableColumnHelper.accessor('data.key', {
    header: 'Key',
    meta: { divider: false },
    cell: ({ table, row: { index } }) => {
      const { workspaceId } = workspaceRoute.useLoaderData();
      return (
        <TextFieldWithVariables
          control={table.options.meta!.control!}
          name={`items.${index}.data.key`}
          workspaceId={workspaceId}
          variant='table-cell'
          className='flex-1'
        />
      );
    },
  }),
  genericFormTableColumnHelper.accessor('data.value', {
    header: 'Value',
    cell: ({ table, row: { index } }) => {
      const { workspaceId } = workspaceRoute.useLoaderData();
      return (
        <TextFieldWithVariables
          control={table.options.meta!.control!}
          name={`items.${index}.data.value`}
          workspaceId={workspaceId}
          variant='table-cell'
          className='flex-1'
        />
      );
    },
  }),
  genericFormTableColumnHelper.accessor('data.description', {
    header: 'Description',
    cell: ({ table, row }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.description`}
        variant='table-cell'
      />
    ),
  }),
];

const genericFormTableColumns = [
  genericFormTableColumnHelper.accessor('data.enabled', {
    header: ({ table }) => <RHFDevTools control={table.options.meta!.control!} className={tw`size-0`} />,
    size: 0,
    cell: ({ table, row }) => (
      <HidePlaceholderCell row={row} table={table} className={tw`flex justify-center`}>
        <CheckboxRHF
          control={table.options.meta!.control!}
          name={`items.${row.index}.data.enabled`}
          variant='table-cell'
        />
      </HidePlaceholderCell>
    ),
  }),
  ...genericFormTableColumnsShared,
  genericFormTableColumnHelper.display({
    id: 'actions',
    header: '',
    size: 0,
    meta: { divider: false },
    cell: ({ table, row }) => (
      <HidePlaceholderCell row={row} table={table}>
        <Button
          className='text-red-700'
          variant='ghost'
          onPress={() => void table.options.meta?.queueTask?.(row.index, 'delete')}
        >
          <LuTrash2 />
        </Button>
      </HidePlaceholderCell>
    ),
  }),
];

export const makeGenericFormTableColumns = <T extends GenericFormTableItem>() =>
  genericFormTableColumns as AccessorKeyColumnDef<TableFormItem<T>>[];

const genericDeltaFormTableColumnHelper = createColumnHelper<DeltaTableFormItem<GenericFormTableItem>>();

const genericDeltaFormTableColumns = [
  genericDeltaFormTableColumnHelper.accessor('data.enabled', {
    header: ({ table }) => <RHFDevTools control={table.options.meta!.control!} className={tw`size-0`} />,
    size: 0,
    cell: ({ table, row }) => (
      <div className={tw`flex justify-center`}>
        <CheckboxRHF
          control={table.options.meta!.control!}
          name={`items.${row.index}.data.enabled`}
          variant='table-cell'
        />
      </div>
    ),
  }),
  ...genericFormTableColumnsShared,
  genericDeltaFormTableColumnHelper.display({
    id: 'actions',
    header: '',
    size: 0,
    meta: { divider: false },
    cell: function ActionCell({ table, row }) {
      const [parentData, data] = useWatch({
        control: table.options.meta!.control!,
        name: [`items.${row.index}.parentData`, `items.${row.index}.data`],
      });

      const parentUlid = pipe(getMessageId(parentData), Option.getOrThrow, (_) => Ulid.construct(_));
      const itemUlid = pipe(getMessageId(data), Option.getOrThrow, (_) => Ulid.construct(_));

      return (
        <Button
          className={twJoin(tw`text-slate-500`, idEqual(parentUlid, itemUlid) && tw`invisible`)}
          variant='ghost'
          onPress={() => void table.options.meta!.queueTask!(row.index, 'undo')}
        >
          <RedoIcon />
        </Button>
      );
    },
  }),
];

export const makeGenericDeltaFormTableColumns = <T extends GenericFormTableItem>() =>
  genericDeltaFormTableColumns as AccessorKeyColumnDef<DeltaTableFormItem<T>>[];
