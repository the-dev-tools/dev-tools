import { AccessorKeyColumnDef, CellContext, DisplayColumnDef, RowData, Table } from '@tanstack/table-core';
import { String } from 'effect';
import { ReactNode, useEffect, useRef } from 'react';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiMove, FiPlus } from 'react-icons/fi';
import { LuTrash2 } from 'react-icons/lu';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { DataTableProps, TableOptions, useReactTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ReferenceField } from '~/features/expression';

interface ReactTableNoMemoProps<TData extends RowData> extends TableOptions<TData> {
  children: (table: Table<TData>) => React.ReactNode;
}

/**
 * Workaround to make React Table work with React Compiler until it's officially supported
 * @see https://github.com/TanStack/table/issues/5567
 */
export const ReactTableNoMemo = <TData extends RowData>({ children, ...props }: ReactTableNoMemoProps<TData>) => {
  'use no memo';
  const table = useReactTable(props);
  return children(table);
};

export interface UseFormTableAddRowProps<TData, TKey extends keyof TData> {
  createLabel?: ReactNode;
  items: TData[];
  onCreate: () => Promise<unknown>;
  primaryColumn?: TKey;
}

export const useFormTableAddRow = <TData, TKey extends keyof TData & string>({
  createLabel = 'New item',
  items,
  onCreate,
  primaryColumn,
}: UseFormTableAddRowProps<TData, TKey>) => {
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
        <FiPlus className={tw`size-4 text-fg-muted`} />
        {createLabel}
      </Button>
    ),
  } satisfies Partial<DataTableProps<TData>>;
};

interface FieldProps<TData, TValue> {
  onChange: (value: TValue, context: CellContext<TData, TValue>) => void;
  value: (provide: (value: TValue) => ReactNode, context: CellContext<TData, TValue>) => ReactNode;
}

export const columnCheckboxField = <TData,>(
  name: keyof TData & string,
  { onChange, value }: FieldProps<TData, boolean>,
  props?: Partial<AccessorKeyColumnDef<TData, boolean>>,
): AccessorKeyColumnDef<TData, boolean> => ({
  accessorKey: name,
  cell: (context) => {
    const provide = (value: boolean) => (
      <Checkbox aria-label={name} isSelected={value} isTableCell onChange={(_) => void onChange(_, context)} />
    );
    return value(provide, context);
  },
  header: '',
  size: 0,
  ...props,
});

export const columnReferenceField = <TData,>(
  name: keyof TData & string,
  { onChange, value }: FieldProps<TData, string>,
  {
    allowFiles,
    title = name,
    ...props
  }: Partial<AccessorKeyColumnDef<TData, string>> & { allowFiles?: boolean; title?: string } = {},
): AccessorKeyColumnDef<TData, string> => ({
  accessorKey: name,
  cell: (context) => {
    const provide = (value: string) => (
      <ReferenceField
        allowFiles={allowFiles}
        className='flex-1'
        kind='StringExpression'
        onChange={(_) => void onChange(_, context)}
        placeholder={`Enter ${title}`}
        value={value}
        variant='table-cell'
      />
    );
    return value(provide, context);
  },
  header: String.capitalize(title),
  ...props,
});

export const columnTextField = <TData,>(
  name: keyof TData & string,
  { onChange, value }: FieldProps<TData, string>,
  { title = name, ...props }: Partial<AccessorKeyColumnDef<TData, string>> & { title?: string } = {},
): AccessorKeyColumnDef<TData, string> => ({
  accessorKey: name,
  cell: (context) => {
    const provide = (value: string) => (
      <TextInputField
        aria-label={title}
        className='flex-1'
        isTableCell
        onChange={(_) => void onChange(_, context)}
        placeholder={`Enter ${title}`}
        value={value}
      />
    );
    return value(provide, context);
  },
  header: String.capitalize(title),
  ...props,
});

export const columnText = <TData,>(
  name: keyof TData & string,
  { title = name, ...props }: Partial<AccessorKeyColumnDef<TData>> & { title?: string } = {},
): AccessorKeyColumnDef<TData> => ({
  accessorKey: name,
  cell: ({ cell }) => <div className={tw`px-5 py-1.5`}>{cell.renderValue() as ReactNode}</div>,
  header: String.capitalize(title),
  ...props,
});

export const columnActions = <T,>({ cell, ...props }: Partial<DisplayColumnDef<T>>): DisplayColumnDef<T> => ({
  cell: (props) => (
    <div className={tw`flex flex-1 justify-end gap-1 px-1`}>{typeof cell === 'function' ? cell(props) : cell}</div>
  ),
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
    <Button className={tw`p-1 text-danger-fg`} onPress={onDelete} variant='ghost'>
      <LuTrash2 />
    </Button>
    <Tooltip className={tw`rounded-md bg-tooltip px-2 py-1 text-xs text-tooltip-fg`}>Delete</Tooltip>
  </TooltipTrigger>
);

export const ColumnActionDrag = () => (
  <Button className={tw`p-1`} slot='drag' variant='ghost'>
    <FiMove className={tw`size-3 text-fg-muted`} />
  </Button>
);

interface ColumnActionsCommonProps<T> {
  onDelete: (item: T) => void;
}

export const columnActionsCommon = <T,>({ onDelete }: ColumnActionsCommonProps<T>) =>
  columnActions<T>({
    cell: ({ row }) => (
      <>
        <ColumnActionDelete onDelete={() => void onDelete(row.original)} />
        <ColumnActionDrag />
      </>
    ),
  });
