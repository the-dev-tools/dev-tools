import { create, Message } from '@bufbuild/protobuf';
import { GenMessage } from '@bufbuild/protobuf/codegenv1';
import { useReactTable } from '@tanstack/react-table';
import {
  AccessorKeyColumnDef,
  ColumnDef,
  createColumnHelper,
  DisplayColumnDef,
  getCoreRowModel,
  RowData,
} from '@tanstack/table-core';
import { Array, HashMap, Option, pipe } from 'effect';
import { idEqual, Ulid } from 'id128';
import { ComponentProps, ReactNode, useCallback, useEffect, useMemo, useRef } from 'react';
import {
  Control,
  FieldPath,
  FieldPathValue,
  FieldPathValues,
  FieldValues,
  Path,
  useFieldArray,
  useForm,
  UseFormGetValues,
  UseFormSetValue,
  UseFormWatch,
  useWatch,
} from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { getMessageId, setMessageId } from '@the-dev-tools/api/meta';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { RHFDevTools } from './dev-tools';
import { TextFieldWithReference } from './reference';

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

export interface FormTableData<T> {
  items: T[];
}

export interface FormTableItem<T> {
  data: T;
}

export interface DeltaFormTableItem<T> extends FormTableItem<T> {
  parentData: T;
}

declare module '@tanstack/table-core' {
  interface TableMeta<TData extends RowData> {
    // Form table column dependencies must be stable to avoid full table re-renders
    // which cause focus loss. Unstable dependencies must be passed via table meta
    queueTask?: (index: number, type: TaskType) => void;
    control?: Control<FormTableData<TData>>;
  }
}

interface GenericFormTableItem extends Message {
  enabled: boolean;
  key: string;
  value: string;
  description: string;
}

const genericFormTableColumnHelper = createColumnHelper<FormTableItem<GenericFormTableItem>>();

export const genericFormTableEnableColumn: AccessorKeyColumnDef<FormTableItem<{ enabled: boolean }>, boolean> = {
  accessorKey: 'data.enabled',
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
};

export const genericFormTableActionColumn: DisplayColumnDef<FormTableItem<GenericFormTableItem>> = {
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
};

const genericFormTableColumnsShared = [
  genericFormTableColumnHelper.accessor('data.key', {
    header: 'Key',
    meta: { divider: false },
    cell: ({ table, row: { index } }) => (
      <TextFieldWithReference
        control={table.options.meta!.control!}
        name={`items.${index}.data.key`}
        variant='table-cell'
        className='flex-1'
      />
    ),
  }),
  genericFormTableColumnHelper.accessor('data.value', {
    header: 'Value',
    cell: ({ table, row: { index } }) => (
      <TextFieldWithReference
        control={table.options.meta!.control!}
        name={`items.${index}.data.value`}
        variant='table-cell'
        className='flex-1'
      />
    ),
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
  genericFormTableEnableColumn,
  ...genericFormTableColumnsShared,
  genericFormTableActionColumn,
];

export const makeGenericFormTableColumns = <T extends GenericFormTableItem>() =>
  genericFormTableColumns as AccessorKeyColumnDef<FormTableItem<T>>[];

const genericDeltaFormTableColumnHelper = createColumnHelper<DeltaFormTableItem<GenericFormTableItem>>();

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
  genericDeltaFormTableColumns as AccessorKeyColumnDef<DeltaFormTableItem<T>>[];

interface UseFormTableProps<T extends Message> {
  columns: ColumnDef<FormTableItem<T>>[];
  schema: GenMessage<T>;
  items: T[];
  onCreate: (item: T) => Promise<Uint8Array>;
  onUpdate: (item: T) => Promise<unknown>;
  onDelete: (item: T) => Promise<unknown>;
}

export const useFormTable = <T extends Message>({
  columns,
  schema,
  items,
  onCreate,
  onUpdate,
  onDelete,
}: UseFormTableProps<T>) => {
  const emptyItem = useCallback(
    (): FormTableItem<T> => ({ data: create(schema, { enabled: true } as object) }),
    [schema],
  );

  const values = useMemo((): FormTableData<FormTableItem<Message>> => {
    return {
      items: pipe(
        Array.map(items, (_) => ({ data: _ })),
        Array.append(emptyItem()),
      ),
    };
  }, [emptyItem, items]);

  const form = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const { queueTask, itemTransaction } = useFieldArrayTasks<
    FormTableData<FormTableItem<Message>>,
    `items.${number}`,
    string,
    FormTableItem<Message>
  >({
    form,
    itemPath: (index) => `items.${index}`,
    itemKey: (_) => pipe(getMessageId(_.data), Option.getOrThrow, (_) => Ulid.construct(_).toCanonical()),
    onTask: async ({ index, item, type }) => {
      const { data } = item as FormTableItem<T>;
      const id = pipe(getMessageId(data), Option.getOrThrow);
      if (type === 'change' && id.length === 0) {
        const newId = await onCreate(data);
        const newIdCan = Ulid.construct(newId).toCanonical();
        itemTransaction(newIdCan, () => {
          form.setValue(`items.${index}.data`, setMessageId(data, newId));
          fieldArray.append(emptyItem());
        });
      } else if (type === 'change') {
        await onUpdate(data);
      } else if (type === 'delete') {
        await onDelete(data);
        itemTransaction(Ulid.construct(id).toCanonical(), () => void fieldArray.remove(index));
      }
    },
  });

  return useReactTable<FormTableItem<Message>>({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => (_ as (typeof fieldArray.fields)[number]).id,
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    meta: { queueTask, control: form.control },
    columns: columns as ColumnDef<FormTableItem<Message>>[],
  });
};

interface UseDeltaFormTableProps<T extends Message> {
  columns: ColumnDef<DeltaFormTableItem<T>>[];
  items: T[];
  deltaItems: T[];
  getParentId: (item: T) => Uint8Array;
  onCreate: (item: T) => Promise<Uint8Array>;
  onUpdate: (item: T) => Promise<unknown>;
  onDelete: (item: T) => Promise<unknown>;
}

export const useDeltaFormTable = <T extends Message>({
  columns,
  items: baseItems,
  deltaItems,
  getParentId,
  onCreate,
  onUpdate,
  onDelete,
}: UseDeltaFormTableProps<T>) => {
  const values = useMemo((): FormTableData<DeltaFormTableItem<Message>> => {
    const deltaItemMap = pipe(
      deltaItems.map((_) => [getParentId(_), _] as const),
      HashMap.fromIterable,
    );

    const items = baseItems.map(
      (_): DeltaFormTableItem<T> => ({
        parentData: _,
        data: pipe(
          getMessageId(_),
          Option.flatMap((id) => HashMap.get(deltaItemMap, id)),
          Option.getOrElse(() => _),
        ),
      }),
    );

    return { items };
  }, [deltaItems, baseItems, getParentId]);

  const form = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const { queueTask, itemTransaction } = useFieldArrayTasks<
    FormTableData<DeltaFormTableItem<Message>>,
    `items.${number}`,
    string,
    DeltaFormTableItem<Message>
  >({
    form,
    itemPath: (index) => `items.${index}`,
    itemKey: (_) => pipe(getMessageId(_.data), Option.getOrThrow, (_) => Ulid.construct(_).toCanonical()),
    onTask: async ({ index, item, type }) => {
      const { data, parentData } = item as DeltaFormTableItem<T>;

      const parentUlid = pipe(getMessageId(parentData), Option.getOrThrow, (_) => Ulid.construct(_));
      const itemUlid = pipe(getMessageId(data), Option.getOrThrow, (_) => Ulid.construct(_));

      if (type === 'change' && idEqual(parentUlid, itemUlid)) {
        const newId = await onCreate(data);
        const newIdCan = Ulid.construct(newId).toCanonical();
        itemTransaction(newIdCan, () => void form.setValue(`items.${index}.data`, setMessageId(data, newId)));
      } else if (type === 'change') {
        await onUpdate(data);
      } else if (type === 'undo') {
        await onDelete(data);
        itemTransaction(parentUlid.toCanonical(), () => void form.setValue(`items.${index}.data`, parentData));
      }
    },
  });

  return useReactTable<DeltaFormTableItem<Message>>({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => (_ as (typeof fieldArray.fields)[number]).id,
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    meta: { queueTask, control: form.control },
    columns: columns as ColumnDef<DeltaFormTableItem<Message>>[],
  });
};
