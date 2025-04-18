import { flexRender, Row, RowData, Table as TanStackTable } from '@tanstack/react-table';
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
  body: tw`divide-y border-inherit`,
  cell: tw`break-all border-inherit align-middle`,
  header: tw`divide-y border-b border-inherit bg-slate-50 font-medium tracking-tight`,
  headerCell: tw`border-inherit px-5 py-1.5 text-left capitalize`,
  row: tw`divide-x border-inherit`,
  table: tw`text-md w-full border-inherit leading-5 text-slate-800`,
  wrapper: tw`overflow-auto rounded-lg border border-slate-200`,
};

export interface DataTableProps<T>
  extends Omit<MixinProps<'wrapper', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'table', ComponentProps<'table'>>, 'children'>,
    Omit<MixinProps<'headerCell', ComponentProps<'th'>>, 'children'>,
    Omit<MixinProps<'header', ComponentProps<'thead'>>, 'children'>,
    Omit<MixinProps<'row', ComponentProps<'tr'>>, 'children'>,
    Omit<MixinProps<'cell', ComponentProps<'td'>>, 'children'>,
    Omit<MixinProps<'body', ComponentProps<'tbody'>>, 'children'> {
  footer?: ReactNode;
  rowRender?: (row: Row<T>, children: ReactNode) => ReactNode;
  table: TanStackTable<T>;
}

export const DataTable = <T,>({
  bodyClassName,
  cellClassName,
  footer,
  headerCellClassName,
  headerCellStyle,
  headerClassName,
  rowClassName,
  rowRender = (_row, _) => _,
  table,
  tableClassName,
  wrapperClassName,
  ...props
}: DataTableProps<T>) => {
  const forwardedProps = splitProps(props, 'wrapper', 'table', 'headerCell', 'header', 'row', 'cell', 'body');

  return (
    <div {...forwardedProps.wrapper} className={twMerge(tableStyles.wrapper, wrapperClassName)}>
      <table {...forwardedProps.table} className={twMerge(tableStyles.table, tableClassName)}>
        <thead {...forwardedProps.header} className={twMerge(tableStyles.header, headerClassName)}>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id} {...forwardedProps.row} className={twMerge(tableStyles.row, rowClassName)}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  {...forwardedProps.headerCell}
                  className={twMerge(
                    tableStyles.headerCell,
                    header.column.columnDef.meta?.divider === false && tw`!border-r-0`,
                    headerCellClassName,
                  )}
                  style={{
                    width: ((header.getSize() / table.getTotalSize()) * 100).toString() + '%',
                    ...headerCellStyle,
                  }}
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody {...forwardedProps.body} className={twMerge(tableStyles.body, bodyClassName)}>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} {...forwardedProps.row} className={twMerge(tableStyles.row, rowClassName)}>
              {rowRender(
                row,
                row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    {...forwardedProps.cell}
                    className={twMerge(
                      tableStyles.cell,
                      cell.column.columnDef.meta?.divider === false && tw`!border-r-0`,
                      cellClassName,
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                )),
              )}
            </tr>
          ))}
        </tbody>
        {footer && (
          <tfoot className={tw`border-inherit`}>
            <tr className={tw`border-inherit`}>
              <td className={tw`border-t border-inherit`} colSpan={table.getAllColumns().length}>
                {footer}
              </td>
            </tr>
          </tfoot>
        )}
      </table>
    </div>
  );
};
