import {
  flexRender,
  getCoreRowModel,
  Row,
  RowData,
  TableOptions as TableOptionsPrimitive,
  Table as TanStackTable,
  useReactTable as useReactTablePrimitive,
} from '@tanstack/react-table';
import { ComponentProps, ReactNode } from 'react';
import { twMerge } from 'tailwind-merge';

import { MixinProps, splitProps } from './mixin-props';
import { tw } from './tailwind-literal';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    divider?: boolean;
  }
}

export const tableStyles = {
  body: tw`col-span-full grid grid-cols-subgrid divide-y border-inherit`,
  cell: tw`block min-w-0 border-inherit align-middle break-all`,
  header: tw`
    col-span-full grid grid-cols-subgrid divide-x border-b border-inherit bg-slate-50 font-medium tracking-tight
  `,
  headerCell: tw`block border-inherit px-5 py-1.5 text-left capitalize`,
  row: tw`col-span-full grid grid-cols-subgrid items-center divide-x border-inherit`,
  table: tw`grid w-full border-inherit text-md leading-5 text-slate-800`,
  wrapper: tw`block overflow-auto rounded-lg border border-slate-200`,
};

export interface TableOptions<TData extends RowData> extends Omit<TableOptionsPrimitive<TData>, 'getCoreRowModel'> {
  getCoreRowModel?: TableOptionsPrimitive<TData>['getCoreRowModel'];
}

export const useReactTable = <TData extends RowData>({ defaultColumn, ...options }: TableOptions<TData>) =>
  useReactTablePrimitive({
    defaultColumn: { minSize: 0, size: 1, ...defaultColumn },
    getCoreRowModel: getCoreRowModel(),
    ...options,
  });

export interface DataTableProps<T>
  extends Omit<MixinProps<'wrapper', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'table', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'headerCell', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'header', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'row', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'cell', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'body', ComponentProps<'div'>>, 'children'> {
  footer?: ReactNode;
  rowRender?: (row: Row<T>, children: ReactNode) => ReactNode;
  table: TanStackTable<T>;
}

export const DataTable = <T,>({
  bodyClassName,
  cellClassName,
  footer,
  headerCellClassName,
  headerClassName,
  rowClassName,
  rowRender = (_row, _) => _,
  table,
  tableClassName,
  wrapperClassName,
  ...props
}: DataTableProps<T>) => {
  const forwardedProps = splitProps(props, 'wrapper', 'table', 'headerCell', 'header', 'row', 'cell', 'body');

  const headerGroups = table.getHeaderGroups();
  if (headerGroups.length !== 1) throw new Error('Header groups not supported');
  const { headers } = headerGroups[0]!;

  return (
    <div {...forwardedProps.wrapper} className={twMerge(tableStyles.wrapper, wrapperClassName)}>
      <div
        {...forwardedProps.table}
        className={twMerge(tableStyles.table, tableClassName)}
        style={{
          gridTemplateColumns: headers
            .map((_) => {
              const size = _.getSize();
              if (size === 0) return 'auto';
              return `${size}fr`;
            })
            .join(' '),
        }}
      >
        {/* Header */}
        <div {...forwardedProps.header} className={twMerge(tableStyles.header, headerClassName)}>
          {headers.map((header) => (
            <div
              key={header.id}
              {...forwardedProps.headerCell}
              className={twMerge(
                tableStyles.headerCell,
                header.column.columnDef.meta?.divider === false && tw`!border-r-0`,
                headerCellClassName,
              )}
            >
              {flexRender(header.column.columnDef.header, header.getContext())}
            </div>
          ))}
        </div>

        {/* Body */}
        <div {...forwardedProps.body} className={twMerge(tableStyles.body, bodyClassName)}>
          {table.getRowModel().rows.map((row) => (
            <div key={row.id} {...forwardedProps.row} className={twMerge(tableStyles.row, rowClassName)}>
              {rowRender(
                row,
                row.getVisibleCells().map((cell) => (
                  <div
                    key={cell.id}
                    {...forwardedProps.cell}
                    className={twMerge(
                      tableStyles.cell,
                      cell.column.columnDef.meta?.divider === false && tw`!border-r-0`,
                      cellClassName,
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </div>
                )),
              )}
            </div>
          ))}
        </div>

        {/* Footer */}
        {footer && <div className={tw`col-span-full border-t border-inherit`}>{footer}</div>}
      </div>
    </div>
  );
};
