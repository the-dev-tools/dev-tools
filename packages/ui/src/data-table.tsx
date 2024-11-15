import { flexRender, RowData, Table as TanStackTable } from '@tanstack/react-table';
import { ComponentProps } from 'react';
import { twMerge } from 'tailwind-merge';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { tw } from './tailwind-literal';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    divider?: boolean;
  }
}

export interface DataTableProps<T>
  extends Omit<MixinProps<'wrapper', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'table', ComponentProps<'table'>>, 'children'>,
    Omit<MixinProps<'headerCell', ComponentProps<'th'>>, 'children'>,
    Omit<MixinProps<'header', ComponentProps<'thead'>>, 'children'>,
    Omit<MixinProps<'row', ComponentProps<'tr'>>, 'children'>,
    Omit<MixinProps<'cell', ComponentProps<'td'>>, 'children'>,
    Omit<MixinProps<'body', ComponentProps<'tbody'>>, 'children'> {
  table: TanStackTable<T>;
}

export const DataTable = <T,>({
  table,
  wrapperClassName,
  tableClassName,
  headerCellClassName,
  headerCellStyle,
  headerClassName,
  rowClassName,
  cellClassName,
  bodyClassName,
  ...props
}: DataTableProps<T>) => {
  const forwardedProps = splitProps(props, 'wrapper', 'table', 'headerCell', 'header', 'row', 'cell', 'body');

  return (
    <div
      {...forwardedProps.wrapper}
      className={twMerge(tw`overflow-auto rounded-lg border border-slate-200`, wrapperClassName)}
    >
      <table
        {...forwardedProps.table}
        className={twMerge(tw`w-full divide-inherit border-inherit text-md leading-5 text-slate-800`, tableClassName)}
      >
        <thead
          {...forwardedProps.header}
          className={twMerge(
            tw`divide-y divide-inherit border-b border-inherit bg-slate-50 font-medium tracking-tight`,
            headerClassName,
          )}
        >
          {table.getHeaderGroups().map((headerGroup) => (
            <tr
              key={headerGroup.id}
              {...forwardedProps.row}
              className={twMerge(tw`divide-x divide-inherit`, rowClassName)}
            >
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  {...forwardedProps.headerCell}
                  className={twMerge(
                    tw`px-5 py-1.5 text-left capitalize`,
                    header.column.columnDef.meta?.divider === false && tw`!border-l-0`,
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
        <tbody {...forwardedProps.body} className={twMerge(tw`divide-y divide-inherit`, bodyClassName)}>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} {...forwardedProps.row} className={twMerge(tw`divide-x divide-inherit`, rowClassName)}>
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  {...forwardedProps.cell}
                  className={twMerge(
                    tw`break-all align-middle`,
                    cell.column.columnDef.meta?.divider === false && tw`!border-l-0`,
                    cellClassName,
                  )}
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
