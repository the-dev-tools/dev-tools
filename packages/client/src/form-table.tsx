import { Message } from '@bufbuild/protobuf';
import { AccessorKeyColumnDef, DisplayColumnDef, RowData, Table } from '@tanstack/table-core';
import { String, Struct } from 'effect';
import { ReactNode, useEffect, useRef } from 'react';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
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

import { SourceKind } from '@the-dev-tools/spec/delta/v1/delta_pb';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTableProps, TableOptions, useReactTable } from '@the-dev-tools/ui/data-table';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { GenericMessage } from '~api/utils';
import { ReferenceFieldRHF } from '~reference';

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

export const makeDeltaItems = <
  T extends Message & {
    origin?: GenericMessage<T>;
    source?: SourceKind;
  },
>(
  items: T[],
  key: keyof T,
) =>
  items.map((_): GenericMessage<T> => {
    if (_.source !== SourceKind.ORIGIN || !_.origin) return _;
    const deltaKey = Struct.pick(_, key, 'source') as Partial<T>;
    return { ..._.origin, ...deltaKey };
  });

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
    return <CheckboxRHF control={control} name={name} variant='table-cell' />;
  },
  header: '',
  size: 0,
  ...props,
});

export const columnReferenceField = <TFieldValues extends FieldValues>(
  name: FieldPath<TFieldValues>,
  {
    allowFiles,
    title = name,
    ...props
  }: Partial<AccessorKeyColumnDef<TFieldValues>> & { allowFiles?: boolean; title?: string } = {},
): AccessorKeyColumnDef<TFieldValues> => ({
  accessorKey: name,
  cell: function Cell() {
    const { control } = useFormContext<TFieldValues>();
    return (
      <ReferenceFieldRHF
        allowFiles={allowFiles}
        className='flex-1'
        control={control}
        name={name}
        placeholder={`Enter ${title}`}
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

interface ColumnActionDeleteProps {
  onDelete: () => void;
}

export const ColumnActionDelete = ({ onDelete }: ColumnActionDeleteProps) => (
  <TooltipTrigger delay={750}>
    <Button className={tw`text-red-700`} onPress={onDelete} variant='ghost'>
      <LuTrash2 />
    </Button>
    <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Delete</Tooltip>
  </TooltipTrigger>
);

interface ColumnActionDeltaResetProps {
  onReset: () => void;
  source: SourceKind | undefined;
}

export const ColumnActionDeltaReset = ({ onReset, source }: ColumnActionDeltaResetProps) => (
  <TooltipTrigger delay={750}>
    <Button
      className={({ isDisabled }) => twJoin(tw`text-slate-500`, isDisabled && tw`invisible`)}
      isDisabled={source !== SourceKind.MIXED}
      onPress={onReset}
      variant='ghost'
    >
      <RedoIcon />
    </Button>
    <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Reset changes</Tooltip>
  </TooltipTrigger>
);

interface ColumnActionsCommonProps<T> {
  onDelete: (item: T) => void;
}

export const columnActionsCommon = <T,>({ onDelete }: ColumnActionsCommonProps<T>) =>
  columnActions<T>({
    cell: ({ row }) => <ColumnActionDelete onDelete={() => void onDelete(row.original)} />,
  });

interface ColumnActionsDeltaCommonProps<T> {
  onDelete: (item: T) => void;
  onReset: (item: T) => void;
  source: (item: T) => SourceKind | undefined;
}

export const columnActionsDeltaCommon = <T,>({ onDelete, onReset, source }: ColumnActionsDeltaCommonProps<T>) =>
  columnActions<T>({
    cell: ({ row }) => (
      <>
        <ColumnActionDeltaReset onReset={() => void onReset(row.original)} source={source(row.original)} />
        <ColumnActionDelete onDelete={() => void onDelete(row.original)} />
      </>
    ),
  });
