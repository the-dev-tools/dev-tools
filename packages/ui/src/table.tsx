import { Array, Option, pipe } from 'effect';
import { ComponentProps, isValidElement, ReactNode } from 'react';
import * as RAC from 'react-aria-components';
import { FiMove } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import { Button } from './button';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

export interface TableProps extends RAC.TableProps {
  containerClassName?: string;
}

export const Table = ({ children, className, containerClassName, ...props }: TableProps) => {
  const footer: ReactNode = pipe(
    Array.ensure(children),
    Array.findFirst((_) => isValidElement(_) && _.type === TableFooter),
    Option.getOrNull,
  );

  return (
    <RAC.ResizableTableContainer
      className={twMerge(tw`relative w-full overflow-auto rounded-lg border border-neutral`, containerClassName)}
      onScroll={props.onScroll}
    >
      <RAC.Table
        className={composeTailwindRenderProps(
          className,
          tw`w-full overflow-hidden border-inherit text-md leading-5 text-on-neutral`,
        )}
        // @ts-expect-error patched workaround until fixed upstream https://github.com/adobe/react-spectrum/issues/2328
        isKeyboardNavigationDisabled
        {...props}
      >
        {children}
      </RAC.Table>

      {footer}
    </RAC.ResizableTableContainer>
  );
};

export interface TableHeaderProps<T> extends RAC.TableHeaderProps<T> {}

export const TableHeader = <T extends object>({ children, className, columns, ...props }: TableHeaderProps<T>) => {
  const { allowsDragging } = RAC.useTableOptions();

  return (
    <RAC.TableHeader
      {...props}
      className={composeTailwindRenderProps(
        className,
        tw`sticky top-0 z-10 border-b border-inherit bg-neutral-lower font-medium tracking-tight`,
      )}
    >
      <RAC.Collection items={columns ?? []}>{children}</RAC.Collection>

      {allowsDragging && <TableColumn width={32} />}
    </RAC.TableHeader>
  );
};

export interface TableColumnProps extends Omit<RAC.ColumnProps, 'className'> {
  className?: string;
}

export const TableColumn = ({ children, className, ...props }: TableColumnProps) => (
  <RAC.Column className={tw`relative border-inherit`} minWidth={0} {...props}>
    {RAC.composeRenderProps(children, (children) => (
      <>
        <div
          className={twMerge(
            tw`box-border flex flex-1 items-center gap-1 overflow-hidden px-5 py-1.5 text-left capitalize`,
            className,
          )}
        >
          {children}
        </div>

        {!props.width && (
          <RAC.ColumnResizer className={tw`absolute inset-y-0 right-0 z-10 translate-x-2 cursor-col-resize px-2`}>
            <div className={tw`mx-auto h-full w-px bg-neutral`} />
          </RAC.ColumnResizer>
        )}
      </>
    ))}
  </RAC.Column>
);

export interface TableBodyProps<T> extends RAC.TableBodyProps<T> {}

export const TableBody = <T extends object>({ className, ...props }: TableBodyProps<T>) => (
  <RAC.TableBody className={composeTailwindRenderProps(className, tw`border-inherit`)} {...props} />
);

export interface TableRowProps<T> extends RAC.RowProps<T> {}

export const TableRow = <T extends object>({ children, className, columns, ...props }: TableRowProps<T>) => {
  const { allowsDragging } = RAC.useTableOptions();

  return (
    <RAC.Row
      className={composeTailwindRenderProps(
        className,
        focusVisibleRingStyles(),
        tw`group/row relative border-inherit -outline-offset-4`,
      )}
      {...props}
    >
      <RAC.Collection items={columns ?? []}>{children}</RAC.Collection>

      {allowsDragging && (
        <TableCell className={tw`cursor-move px-1`}>
          <Button className={tw`p-1`} slot='drag' variant='ghost'>
            <FiMove className={tw`size-3 text-on-neutral-low`} />
          </Button>
        </TableCell>
      )}
    </RAC.Row>
  );
};

export interface TableCellProps extends RAC.CellProps {}

export const TableCell = ({ className, ...props }: TableCellProps) => (
  <RAC.Cell
    className={composeTailwindRenderProps(
      className,
      focusVisibleRingStyles(),
      tw`border-r border-b border-inherit align-middle break-all select-text group-last/row:border-b-0 last:border-r-0`,
    )}
    {...props}
  />
);

export interface TableFooterProps extends ComponentProps<'div'> {}

export const TableFooter = ({ className, ...props }: TableFooterProps) => (
  <div className={twMerge(tw`border-t border-inherit`, className)} {...props} />
);
