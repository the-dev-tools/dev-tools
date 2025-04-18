import { DescMessage, DescMethodUnary, Message, MessageInitShape } from '@bufbuild/protobuf';
import { useReactTable } from '@tanstack/react-table';
import {
  AccessorKeyColumnDef,
  ColumnDef,
  createColumnHelper,
  DisplayColumnDef,
  getCoreRowModel,
  RowData,
} from '@tanstack/table-core';
import { Array, HashMap, Option, pipe, String } from 'effect';
import { idEqual, Ulid } from 'id128';
import { ComponentProps, ReactNode, useCallback, useEffect, useMemo, useRef } from 'react';
import {
  Control,
  FieldPath,
  FieldPathValue,
  FieldPathValues,
  FieldValues,
  FormProvider,
  Path,
  useFieldArray,
  useForm,
  useFormContext,
  UseFormGetValues,
  UseFormHandleSubmit,
  UseFormSetValue,
  UseFormWatch,
  useWatch,
} from 'react-hook-form';
import { FiPlus } from 'react-icons/fi';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { Button } from '@the-dev-tools/ui/button';
import { Checkbox, CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTableProps } from '@the-dev-tools/ui/data-table';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { inputStyles, TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { getMessageId, setMessageId } from '~/api/meta';
import { useConnectMutation } from '~api/connect-query';

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
}: HidePlaceholderCellProps) => {
  // eslint-disable-next-line react-compiler/react-compiler
  'use no memo';
  return <div {...props} className={twJoin(className, index + 1 === getRowCount() && tw`invisible`)} />;
};

interface FormWatchProps<
  TFieldValues extends FieldValues = FieldValues,
  TFieldNames extends readonly FieldPath<TFieldValues>[] = readonly FieldPath<TFieldValues>[],
> {
  children: (values: FieldPathValues<TFieldValues, TFieldNames>) => ReactNode;
  control: Control<TFieldValues>;
  name: readonly [...TFieldNames];
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
  itemKey: (item: TItem) => TKey;
  itemPath: (index: number) => TItemPath;
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
  itemKey,
  itemPath,
  onTask,
  wait = 200,
}: UseFieldArrayTasksProps<TFieldValues, TItemPath, TKey, TItem>) => {
  const isPending = useRef(false);
  const tasks = useRef(new Map<TKey, Task<TItem>>());
  const ignoreChanges = useRef(new Set<TKey>());

  const itemTransaction = useCallback(
    (key: TKey, callback: () => void) => {
      ignoreChanges.current.add(key);
      callback();
      ignoreChanges.current.delete(key);
    },
    [ignoreChanges],
  );

  const processTasks = useDebouncedCallback(async () => {
    // Wait for all mutations to finish before processing new updates
    if (isPending.current) return void processTasks();
    isPending.current = true;

    await pipe(
      Array.fromIterable(tasks.current),
      Array.map(async ([key, task]) => {
        await onTask(task);
        const nextTask = tasks.current.get(key);
        if (!nextTask) return;
        if (nextTask.index === task.index && nextTask.type === task.type) tasks.current.delete(key);
      }),
      (_) => Promise.allSettled(_),
    );

    isPending.current = false;
  }, wait);

  const queueTask = useCallback(
    (index: number, type: TaskType) => {
      const item = form.getValues(itemPath(index));
      const key = itemKey(item);

      if (ignoreChanges.current.has(key)) return;

      tasks.current.set(key, { index, item, type });
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

  return { itemTransaction, queueTask };
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
    control?: Control<FormTableData<TData>>;
    // Form table column dependencies must be stable to avoid full table re-renders
    // which cause focus loss. Unstable dependencies must be passed via table meta
    queueTask?: (index: number, type: TaskType) => void;
  }
}

interface GenericFormTableItem extends Message {
  description: string;
  enabled: boolean;
  key: string;
  value: string;
}

const genericDisplayTableColumnHelper = createColumnHelper<GenericFormTableItem>();

const genericDisplayTableColumns = [
  genericDisplayTableColumnHelper.accessor('enabled', {
    cell: ({ getValue }) => (
      <div className={tw`flex justify-center`}>
        <Checkbox isReadOnly isSelected={getValue()} variant='table-cell' />
      </div>
    ),
    header: () => null,
    size: 0,
  }),
  genericDisplayTableColumnHelper.accessor('key', {
    cell: ({ getValue }) => <div className={inputStyles({ variant: 'table-cell' })}>{getValue()}</div>,
    header: 'Key',
    meta: { divider: false },
  }),
  genericDisplayTableColumnHelper.accessor('value', {
    cell: ({ getValue }) => <div className={inputStyles({ variant: 'table-cell' })}>{getValue()}</div>,
    header: 'Value',
  }),
  genericDisplayTableColumnHelper.accessor('description', {
    cell: ({ getValue }) => <div className={inputStyles({ variant: 'table-cell' })}>{getValue()}</div>,
    header: 'Description',
  }),
];

export const makeGenericDisplayTableColumns = <T extends GenericFormTableItem>() =>
  genericDisplayTableColumns as AccessorKeyColumnDef<T>[];

const genericFormTableColumnHelper = createColumnHelper<FormTableItem<GenericFormTableItem>>();

export const genericFormTableEnableColumn: AccessorKeyColumnDef<FormTableItem<{ enabled: boolean }>, boolean> = {
  accessorKey: 'data.enabled',
  cell: ({ row, table }) => (
    <HidePlaceholderCell className={tw`flex justify-center`} row={row} table={table}>
      <CheckboxRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.enabled`}
        variant='table-cell'
      />
    </HidePlaceholderCell>
  ),
  header: ({ table }) => <RHFDevTools className={tw`size-0`} control={table.options.meta!.control!} />,
  size: 0,
};

export const genericFormTableActionColumn: DisplayColumnDef<FormTableItem<GenericFormTableItem>> = {
  cell: ({ row, table }) => (
    <HidePlaceholderCell row={row} table={table}>
      <Button
        className='text-red-700'
        onPress={() => void table.options.meta?.queueTask?.(row.index, 'delete')}
        variant='ghost'
      >
        <LuTrash2 />
      </Button>
    </HidePlaceholderCell>
  ),
  header: '',
  id: 'actions',
  meta: { divider: false },
  size: 0,
};

const genericFormTableColumnsShared = [
  genericFormTableColumnHelper.accessor('data.key', {
    cell: ({ row: { index }, table }) => (
      <TextFieldWithReference
        className='flex-1'
        control={table.options.meta!.control!}
        inputPlaceholder='Enter key'
        name={`items.${index}.data.key`}
        variant='table-cell'
      />
    ),
    header: 'Key',
    meta: { divider: false },
  }),
  genericFormTableColumnHelper.accessor('data.value', {
    cell: ({ row: { index }, table }) => (
      <TextFieldWithReference
        className='flex-1'
        control={table.options.meta!.control!}
        inputPlaceholder='Enter value'
        name={`items.${index}.data.value`}
        variant='table-cell'
      />
    ),
    header: 'Value',
  }),
  genericFormTableColumnHelper.accessor('data.description', {
    cell: ({ row, table }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        inputPlaceholder='Enter description'
        name={`items.${row.index}.data.description`}
        variant='table-cell'
      />
    ),
    header: 'Description',
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
    cell: ({ row, table }) => (
      <div className={tw`flex justify-center`}>
        <CheckboxRHF
          control={table.options.meta!.control!}
          name={`items.${row.index}.data.enabled`}
          variant='table-cell'
        />
      </div>
    ),
    header: ({ table }) => <RHFDevTools className={tw`size-0`} control={table.options.meta!.control!} />,
    size: 0,
  }),
  ...genericFormTableColumnsShared,
  genericDeltaFormTableColumnHelper.display({
    cell: function ActionCell({ row, table }) {
      const [parentData, data] = useWatch({
        control: table.options.meta!.control!,
        name: [`items.${row.index}.parentData`, `items.${row.index}.data`],
      });

      const parentUlid = pipe(getMessageId(parentData), Option.getOrThrow, (_) => Ulid.construct(_));
      const itemUlid = pipe(getMessageId(data), Option.getOrThrow, (_) => Ulid.construct(_));

      return (
        <Button
          className={twJoin(tw`text-slate-500`, idEqual(parentUlid, itemUlid) && tw`invisible`)}
          onPress={() => void table.options.meta!.queueTask!(row.index, 'undo')}
          variant='ghost'
        >
          <RedoIcon />
        </Button>
      );
    },
    header: '',
    id: 'actions',
    meta: { divider: false },
    size: 0,
  }),
];

export const makeGenericDeltaFormTableColumns = <T extends GenericFormTableItem>() =>
  genericDeltaFormTableColumns as AccessorKeyColumnDef<DeltaFormTableItem<T>>[];

interface UseDeltaFormTableProps<T extends Message> {
  columns: ColumnDef<DeltaFormTableItem<T>>[];
  deltaItems: T[];
  getParentId: (item: T) => Uint8Array;
  items: T[];
  onCreate: (item: T) => Promise<Uint8Array>;
  onDelete: (item: T) => Promise<unknown>;
  onUpdate: (item: T) => Promise<unknown>;
}

export const useDeltaFormTable = <T extends Message>({
  columns,
  deltaItems,
  getParentId,
  items: baseItems,
  onCreate,
  onDelete,
  onUpdate,
}: UseDeltaFormTableProps<T>) => {
  const values = useMemo((): FormTableData<DeltaFormTableItem<Message>> => {
    const deltaItemMap = pipe(
      deltaItems.map((_) => [getParentId(_).toString(), _] as const),
      HashMap.fromIterable,
    );

    const items = baseItems.map(
      (_): DeltaFormTableItem<T> => ({
        data: pipe(
          getMessageId(_),
          Option.flatMap((id) => HashMap.get(deltaItemMap, id.toString())),
          Option.getOrElse(() => _),
        ),
        parentData: _,
      }),
    );

    return { items };
  }, [deltaItems, baseItems, getParentId]);

  const form = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const { itemTransaction, queueTask } = useFieldArrayTasks<
    FormTableData<DeltaFormTableItem<Message>>,
    `items.${number}`,
    string,
    DeltaFormTableItem<Message>
  >({
    form,
    itemKey: (_) => pipe(getMessageId(_.data), Option.getOrThrow, (_) => Ulid.construct(_).toCanonical()),
    itemPath: (index) => `items.${index}`,
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
    columns: columns as ColumnDef<DeltaFormTableItem<Message>>[],
    data: fieldArray.fields,
    defaultColumn: { minSize: 0 },
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => (_ as (typeof fieldArray.fields)[number]).id,
    meta: { control: form.control, queueTask },
  });
};

interface UseFormAutoSaveProps<TFieldValues extends FieldValues> {
  handleSubmit: UseFormHandleSubmit<TFieldValues>;
  onSubmit: (value: TFieldValues) => Promise<unknown>;
  watch: UseFormWatch<TFieldValues>;
}

const useFormAutoSave = <TFieldValues extends FieldValues>({
  handleSubmit,
  onSubmit,
  watch,
}: UseFormAutoSaveProps<TFieldValues>) => {
  const submit = useDebouncedCallback(async () => handleSubmit((value) => onSubmit(value))(), 200);

  useEffect(
    () =>
      watch((_, { type }) => {
        if (type === 'change') void submit();
      }).unsubscribe,
    [submit, watch],
  );
};

interface FormTableRowProps<T extends FieldValues> {
  children: ReactNode;
  onUpdate: (value: T) => Promise<unknown>;
  value: T;
}

const FormTableRow = <T extends FieldValues>({ children, onUpdate, value }: FormTableRowProps<T>) => {
  const form = useForm({ values: value });
  useFormAutoSave({ ...form, onSubmit: onUpdate });
  return <FormProvider {...form}>{children}</FormProvider>;
};

interface UseFormTableProps<TFieldValues extends FieldValues, TPrimaryName extends FieldPath<TFieldValues>> {
  createLabel?: ReactNode;
  items: TFieldValues[];
  onCreate: () => Promise<unknown>;
  onUpdate: (value: TFieldValues) => Promise<unknown>;
  primaryColumn?: TPrimaryName;
}

export const useFormTable = <TFieldValues extends FieldValues, TPrimaryName extends FieldPath<TFieldValues>>({
  createLabel = 'New item',
  items,
  onCreate,
  onUpdate,
  primaryColumn,
}: UseFormTableProps<TFieldValues, TPrimaryName>) => {
  const lengthPrev = useRef<null | number>(null);

  useEffect(() => {
    if (!primaryColumn || !bodyRef.current || lengthPrev.current === null || lengthPrev.current === items.length)
      return;

    const lastRow = bodyRef.current.children.item(items.length - 1);
    const primaryCell = lastRow?.querySelector(`[name="${primaryColumn}"]`);
    if (primaryCell instanceof HTMLElement) primaryCell.focus();

    lengthPrev.current = null;
  });

  const bodyRef = useRef<HTMLTableSectionElement>(null);

  return {
    bodyRef,
    footer: (
      <Button
        className={tw`w-full justify-start rounded-none -outline-offset-4`}
        onPress={async () => {
          await onCreate();
          lengthPrev.current = items.length;
        }}
        variant='ghost'
      >
        <FiPlus className={tw`size-4 text-slate-500`} />
        {createLabel}
      </Button>
    ),
    rowRender: (row, _) => (
      <FormTableRow onUpdate={onUpdate} value={row.original}>
        {_}
      </FormTableRow>
    ),
  } satisfies Partial<DataTableProps<TFieldValues>>;
};

export const columnCheckboxField = <TFieldValues extends FieldValues>(
  name: FieldPath<TFieldValues>,
  props?: Partial<AccessorKeyColumnDef<TFieldValues>>,
): AccessorKeyColumnDef<TFieldValues> => ({
  accessorKey: name,
  cell: function Cell() {
    const { control } = useFormContext<TFieldValues>();
    return (
      <div className={tw`flex justify-center`}>
        <CheckboxRHF control={control} name={name} variant='table-cell' />
      </div>
    );
  },
  header: '',
  size: 0,
  ...props,
});

export const columnTextFieldWithReference = <TFieldValues extends FieldValues>(
  name: FieldPath<TFieldValues>,
  { title = name, ...props }: Partial<AccessorKeyColumnDef<TFieldValues>> & { title?: string } = {},
): AccessorKeyColumnDef<TFieldValues> => ({
  accessorKey: name,
  cell: function Cell() {
    const { control } = useFormContext<TFieldValues>();
    return (
      <TextFieldWithReference
        className='flex-1'
        control={control}
        inputPlaceholder={`Enter ${title}`}
        name={name}
        variant='table-cell'
      />
    );
  },
  header: String.capitalize(title),
  ...props,
});

export const columnTextField = <TFieldValues extends FieldValues>(
  name: FieldPath<TFieldValues>,
  { title = name, ...props }: Partial<AccessorKeyColumnDef<TFieldValues>> & { title?: string } = {},
): AccessorKeyColumnDef<TFieldValues> => ({
  accessorKey: name,
  cell: function Cell() {
    const { control } = useFormContext<TFieldValues>();
    return (
      <TextFieldRHF
        className='flex-1'
        control={control}
        inputPlaceholder={`Enter ${title}`}
        name={name}
        variant='table-cell'
      />
    );
  },
  header: String.capitalize(title),
  ...props,
});

export const columnActions = <T,>({ cell, ...props }: Partial<DisplayColumnDef<T>>): DisplayColumnDef<T> => ({
  cell: (props) => <div className={tw`flex justify-center`}>{typeof cell === 'function' ? cell(props) : cell}</div>,
  header: '',
  id: 'actions',
  size: 0,
  ...props,
});

interface ColumnActionDeleteProps<I extends DescMessage, O extends DescMessage> {
  input: MessageInitShape<I>;
  schema: DescMethodUnary<I, O>;
}

export const ColumnActionDelete = <I extends DescMessage, O extends DescMessage>({
  input,
  schema,
}: ColumnActionDeleteProps<I, O>) => {
  const delete$ = useConnectMutation(schema);
  return (
    <Button className={tw`text-red-700`} onPress={() => void delete$.mutateAsync(input)} variant='ghost'>
      <LuTrash2 />
    </Button>
  );
};
