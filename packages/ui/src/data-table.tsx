import {
  flexRender,
  getCoreRowModel,
  Row,
  RowData,
  TableOptions as TableOptionsPrimitive,
  Table as TanStackTable,
  useReactTable as useReactTablePrimitive,
} from '@tanstack/react-table';
import { pipe } from 'effect';
import { ComponentProps, ReactNode, RefAttributes, useEffect, useRef } from 'react';
import {
  Cell as AriaCell,
  CellProps as AriaCellProps,
  Column as AriaColumn,
  ColumnProps as AriaColumnProps,
  Row as AriaRow,
  RowProps as AriaRowProps,
  Table as AriaTable,
  TableBody as AriaTableBody,
  TableBodyProps as AriaTableBodyProps,
  TableHeader as AriaTableHeader,
  TableHeaderProps as AriaTableHeaderProps,
  TableProps as AriaTableProps,
} from 'react-aria-components';
import { twJoin, twMerge } from 'tailwind-merge';
import { MixinProps, splitProps } from './mixin-props';
import { tw } from './tailwind-literal';
import { composeRenderPropsTW } from './utils';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    divider?: boolean;
    isRowHeader?: boolean;
  }
}

export const tableStyles = {
  body: tw`col-span-full grid grid-cols-subgrid divide-y border-inherit`,
  cell: tw`block min-w-0 border-inherit align-middle break-all`,
  header: tw`
    col-span-full grid grid-cols-subgrid divide-x border-b border-inherit bg-slate-50 font-medium tracking-tight

    *:contents
  `,
  headerColumn: tw`block border-inherit px-5 py-1.5 text-left capitalize`,
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

export interface CellRenderProps {
  cellNode: ReactNode;
}
export type CellRender = (props: CellRenderProps) => ReactNode;
export interface RowRenderProps<T> {
  row: Row<T>;
  rowNode: (cellRender?: CellRender) => ReactNode;
}
export type RowRender<T> = (props: RowRenderProps<T>) => ReactNode;

export interface DataTableProps<T>
  extends Omit<MixinProps<'wrapper', ComponentProps<'div'>>, 'children'>,
    Omit<MixinProps<'table', AriaTableProps>, 'children'>,
    Omit<MixinProps<'headerColumn', AriaColumnProps>, 'children'>,
    Omit<MixinProps<'header', AriaTableHeaderProps<T>>, 'children'>,
    Omit<MixinProps<'row', AriaRowProps<T>>, 'children'>,
    Omit<MixinProps<'cell', AriaCellProps>, 'children'>,
    Omit<MixinProps<'body', AriaTableBodyProps<T> & RefAttributes<HTMLTableSectionElement>>, 'children'> {
  footer?: ReactNode;
  rowRender?: RowRender<T>;
  table: TanStackTable<T>;
}

export const DataTable = <T extends object>({
  bodyClassName,
  cellClassName,
  footer,
  headerClassName,
  headerColumnClassName,
  rowClassName,
  rowRender = ({ rowNode }) => rowNode(),
  table,
  tableClassName,
  wrapperClassName,
  ...props
}: DataTableProps<T>) => {
  const forwardedProps = splitProps(props, 'wrapper', 'table', 'headerColumn', 'header', 'row', 'cell', 'body');

  // Disable key propagation when table is focused to allow input fields in table
  // https://github.com/adobe/react-spectrum/issues/4674#issuecomment-1667722934
  const isFocused = useRef(false);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (isFocused.current) e.stopPropagation();
    };
    window.addEventListener('keydown', handler, true);
    return () => void window.removeEventListener('keydown', handler, true);
  }, []);

  const headerGroups = table.getHeaderGroups();
  if (headerGroups.length !== 1) throw new Error('Header groups not supported');
  const { headers } = headerGroups[0]!;

  return (
    <div {...forwardedProps.wrapper} className={twMerge(tableStyles.wrapper, wrapperClassName)}>
      <AriaTable
        ref={(ref) => {
          const handleFocusIn = () => (isFocused.current = true);
          const handleFocusOut = () => (isFocused.current = false);

          ref?.addEventListener('focusin', handleFocusIn);
          ref?.addEventListener('focusout', handleFocusOut);

          return () => {
            ref?.removeEventListener('focusin', handleFocusIn);
            ref?.removeEventListener('focusout', handleFocusOut);
          };
        }}
        {...forwardedProps.table}
        className={composeRenderPropsTW(tableClassName, tableStyles.table)}
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
        <AriaTableHeader
          {...forwardedProps.header}
          className={composeRenderPropsTW(headerClassName, tableStyles.header)}
        >
          {headers.map((header) => (
            <AriaColumn
              key={header.id}
              {...forwardedProps.headerColumn}
              className={composeRenderPropsTW(
                headerColumnClassName,
                twJoin(tableStyles.headerColumn, header.column.columnDef.meta?.divider === false && tw`!border-r-0`),
              )}
              isRowHeader={header.column.columnDef.meta?.isRowHeader ?? false}
            >
              {flexRender(header.column.columnDef.header, header.getContext())}
            </AriaColumn>
          ))}
        </AriaTableHeader>

        {/* Body */}
        <AriaTableBody {...forwardedProps.body} className={composeRenderPropsTW(bodyClassName, tableStyles.body)}>
          {table.getRowModel().rows.map((row) => (
            <AriaRow
              key={row.id}
              {...forwardedProps.row}
              className={composeRenderPropsTW(rowClassName, tableStyles.row)}
            >
              {rowRender({
                row,
                rowNode: (cellRender = ({ cellNode }) => cellNode) =>
                  row.getVisibleCells().map((cell) => (
                    <AriaCell
                      key={cell.id}
                      {...forwardedProps.cell}
                      className={composeRenderPropsTW(
                        cellClassName,
                        twJoin(tableStyles.cell, cell.column.columnDef.meta?.divider === false && tw`!border-r-0`),
                      )}
                    >
                      {pipe(
                        cell.getContext(),
                        (_) => flexRender(cell.column.columnDef.cell, _),
                        (_) => cellRender({ cellNode: _ }),
                      )}
                    </AriaCell>
                  )),
              })}
            </AriaRow>
          ))}

          {/* Footer workaround, as at the moment proper footer is not supported */}
          {/* https://github.com/adobe/react-spectrum/issues/4372 */}
          {footer && (
            <AriaRow className={composeRenderPropsTW(rowClassName, tableStyles.row)}>
              <AriaCell
                className={composeRenderPropsTW(cellClassName, twJoin(tableStyles.cell, tw`col-span-full`))}
                colSpan={headers.length}
              >
                {footer}
              </AriaCell>
            </AriaRow>
          )}
        </AriaTableBody>
      </AriaTable>
    </div>
  );
};
