import * as TSR from '@tanstack/react-table';
import { pipe } from 'effect';
import { ReactNode } from 'react';
import * as RAC from 'react-aria-components';
import { twJoin } from 'tailwind-merge';
import { tv } from 'tailwind-variants';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends TSR.RowData, TValue> {
    divider?: boolean;
    isRowHeader?: boolean;
  }
}

export const tableStyles = tv({
  slots: {
    base: tw`grid w-full border-inherit text-md leading-5 text-slate-800`,
    body: tw`col-span-full grid grid-cols-subgrid divide-y border-inherit`,
    cell: tw`block min-w-0 border-inherit align-middle break-all select-text`,
    container: tw`block overflow-auto rounded-lg border border-slate-200`,
    footer: tw`
      col-span-full block min-w-0 items-center divide-x border-t border-inherit align-middle break-all select-text
    `,
    header: tw`
      col-span-full grid grid-cols-subgrid divide-x border-b border-inherit bg-slate-50 font-medium tracking-tight

      *:contents
    `,
    headerColumn: tw`block border-inherit px-5 py-1.5 text-left capitalize`,
    row: tw`col-span-full grid grid-cols-subgrid items-center divide-x border-inherit`,
  },
});

export interface TableOptions<TData extends TSR.RowData> extends Omit<TSR.TableOptions<TData>, 'getCoreRowModel'> {
  getCoreRowModel?: TSR.TableOptions<TData>['getCoreRowModel'];
}

export const useReactTable = <TData extends TSR.RowData>({ defaultColumn, ...options }: TableOptions<TData>) =>
  TSR.useReactTable({
    defaultColumn: { minSize: 0, size: 1, ...defaultColumn },
    getCoreRowModel: TSR.getCoreRowModel(),
    ...options,
  });

export interface CellRenderProps {
  cellNode: ReactNode;
}
export type CellRender = (props: CellRenderProps) => ReactNode;
export interface RowRenderProps<T> {
  row: TSR.Row<T>;
  rowNode: (cellRender?: CellRender) => ReactNode;
}
export type RowRender<T> = (props: RowRenderProps<T>) => ReactNode;

export interface DataTableProps<T> extends RAC.TableProps {
  footer?: ReactNode;
  rowRender?: RowRender<T>;
  table: TSR.Table<T>;
}

export const DataTable = <T extends object>({
  className,
  footer,
  rowRender = ({ rowNode }) => rowNode(),
  table,
  ...props
}: DataTableProps<T>) => {
  const headerGroups = table.getHeaderGroups();
  if (headerGroups.length !== 1) throw new Error('Header groups not supported');
  const { headers } = headerGroups[0]!;

  const styles = tableStyles(props);

  return (
    <div className={styles.container()}>
      <RAC.Table
        // @ts-expect-error patched workaround until fixed upstream https://github.com/adobe/react-spectrum/issues/2328
        isKeyboardNavigationDisabled
        {...props}
        className={composeStyleRenderProps(className, styles.base)}
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
        <RAC.TableHeader className={styles.header()}>
          {headers.map((header) => (
            <RAC.Column
              className={styles.headerColumn({
                className: header.column.columnDef.meta?.divider === false && tw`!border-r-0`,
              })}
              id={header.id}
              isRowHeader={header.column.columnDef.meta?.isRowHeader ?? false}
              key={header.id}
            >
              {TSR.flexRender(header.column.columnDef.header, header.getContext())}
            </RAC.Column>
          ))}
        </RAC.TableHeader>

        {/* Body */}
        <RAC.TableBody className={styles.body()}>
          {table.getRowModel().rows.map((row) => (
            <RAC.Row className={styles.row()} id={row.id} key={row.id}>
              {rowRender({
                row,
                rowNode: (cellRender = ({ cellNode }) => cellNode) =>
                  row.getVisibleCells().map((cell) => (
                    <RAC.Cell
                      className={styles.cell({
                        className: twJoin(cell.column.columnDef.meta?.divider === false && tw`!border-r-0`),
                      })}
                      id={cell.id}
                      key={cell.id}
                    >
                      {pipe(
                        cell.getContext(),
                        (_) => TSR.flexRender(cell.column.columnDef.cell, _),
                        (_) => cellRender({ cellNode: _ }),
                      )}
                    </RAC.Cell>
                  )),
              })}
            </RAC.Row>
          ))}
        </RAC.TableBody>
      </RAC.Table>

      {/* Footer workaround, as at the moment proper footer is not supported */}
      {/* https://github.com/adobe/react-spectrum/issues/4372 */}
      {footer && <div className={styles.footer()}>{footer}</div>}
    </div>
  );
};
