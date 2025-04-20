import { DescMessage, DescMethodUnary, MessageInitShape } from '@bufbuild/protobuf';
import { useReactTable } from '@tanstack/react-table';
import { AccessorKeyColumnDef, DisplayColumnDef, RowData, Table, TableOptions } from '@tanstack/table-core';
import { HashMap, Option, pipe, String } from 'effect';
import { ReactNode, useEffect, useRef } from 'react';
import {
  FieldPath,
  FieldValues,
  FormProvider,
  useForm,
  useFormContext,
  UseFormHandleSubmit,
  UseFormWatch,
} from 'react-hook-form';
import { FiPlus } from 'react-icons/fi';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { useDebouncedCallback } from 'use-debounce';

import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTableProps } from '@the-dev-tools/ui/data-table';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { useConnectMutation } from '~api/connect-query';

import { TextFieldWithReference } from './reference';

interface ReactTableNoMemoProps<TData extends RowData> extends TableOptions<TData> {
  children: (table: Table<TData>) => React.ReactNode;
}

/**
 * Workaround to make React Table work with React Compiler until it's officially supported
 * @see https://github.com/TanStack/table/issues/5567
 */
export const ReactTableNoMemo = <TData extends RowData>({ children, ...props }: ReactTableNoMemoProps<TData>) => {
  // eslint-disable-next-line react-compiler/react-compiler
  'use no memo';
  const table = useReactTable(props);
  return children(table);
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

interface UseDeltaItemsProps<TFieldValues extends FieldValues> {
  getId: (item: TFieldValues) => string;
  getParentId: (item: TFieldValues) => string | undefined;
  itemsBase: TFieldValues[];
  itemsDelta: TFieldValues[];
}

export const useDeltaItems = <TFieldValues extends FieldValues>({
  getId,
  getParentId,
  itemsBase,
  itemsDelta,
}: UseDeltaItemsProps<TFieldValues>) => {
  const deltaItemMap = pipe(
    itemsDelta.map((_) => [getParentId(_), _] as const),
    HashMap.fromIterable,
  );

  return itemsBase.map((_) =>
    pipe(
      HashMap.get(deltaItemMap, getId(_)),
      Option.getOrElse(() => _),
    ),
  );
};

interface UseDeltaFormTableProps<TFieldValues extends FieldValues> {
  getParentId: (item: TFieldValues) => string | undefined;
  onCreate: (value: TFieldValues) => Promise<unknown>;
  onUpdate: (value: TFieldValues) => Promise<unknown>;
}

export const useDeltaFormTable = <TFieldValues extends FieldValues>({
  getParentId,
  onCreate,
  onUpdate,
}: UseDeltaFormTableProps<TFieldValues>) =>
  ({
    rowRender: (row, _) => (
      <FormTableRow
        onUpdate={async (data) => {
          if (getParentId(data) !== undefined) await onUpdate(data);
          else await onCreate(data);
        }}
        value={row.original}
      >
        {_}
      </FormTableRow>
    ),
  }) satisfies Partial<DataTableProps<TFieldValues>>;

interface DisplayFormTableRowProps<T extends FieldValues> {
  children: ReactNode;
  value: T;
}

const DisplayFormTableRow = <T extends FieldValues>({ children, value }: DisplayFormTableRowProps<T>) => {
  const form = useForm({ disabled: true, values: value });
  return <FormProvider {...form}>{children}</FormProvider>;
};

export const displayTable = {
  rowRender: (row, _) => <DisplayFormTableRow value={row.original}>{_}</DisplayFormTableRow>,
} satisfies Partial<DataTableProps<FieldValues>>;

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

interface ColumnActionUndoDeltaProps<I extends DescMessage, O extends DescMessage> {
  hasDelta: boolean;
  input: MessageInitShape<I>;
  schema: DescMethodUnary<I, O>;
}

export const ColumnActionUndoDelta = <I extends DescMessage, O extends DescMessage>({
  hasDelta,
  input,
  schema,
}: ColumnActionUndoDeltaProps<I, O>) => {
  const delete$ = useConnectMutation(schema);
  return (
    <Button
      className={({ isDisabled }) => twJoin(tw`text-slate-500`, isDisabled && tw`invisible`)}
      isDisabled={!hasDelta}
      onPress={() => void delete$.mutateAsync(input)}
      variant='ghost'
    >
      <RedoIcon />
    </Button>
  );
};
