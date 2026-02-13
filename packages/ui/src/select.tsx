import { RefAttributes } from 'react';
import * as RAC from 'react-aria-components';
import { FiCheckCircle, FiChevronDown } from 'react-icons/fi';
import { Button, ButtonProps } from './button';
import { FieldError, type FieldErrorProps, FieldLabel, type FieldLabelProps } from './field';
import { ListBox, ListBoxItem, ListBoxItemProps, ListBoxProps } from './list-box';
import { Popover } from './popover';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps, composeTextValueProps } from './utils';

// Root

export interface SelectProps<T extends object, M extends 'multiple' | 'single' = 'single'>
  extends
    Omit<RAC.SelectProps<T, M>, 'children'>,
    Pick<ListBoxProps<T>, 'children' | 'items'>,
    RefAttributes<HTMLDivElement> {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
  renderValue?: RAC.SelectValueProps<T>['children'];
  triggerClassName?: ButtonProps['className'];
  triggerVariant?: ButtonProps['variant'];
}

export const Select = <T extends object>({
  children,
  className,
  error,
  items,
  label,
  renderValue,
  triggerClassName,
  triggerVariant,
  ...props
}: SelectProps<T>) => (
  <RAC.Select {...props} className={composeTailwindRenderProps(className, tw`group flex flex-col gap-1`)}>
    {label && <FieldLabel>{label}</FieldLabel>}
    <Button className={triggerClassName!} variant={triggerVariant}>
      <RAC.SelectValue>{renderValue}</RAC.SelectValue>
      <FiChevronDown className={tw`-mr-1 size-4 text-on-neutral-low transition-transform group-open:rotate-180`} />
    </Button>
    {error && <FieldError>{error}</FieldError>}
    <Popover>
      <ListBox items={items!}>{children}</ListBox>
    </Popover>
  </RAC.Select>
);

// Item

export interface SelectItemProps<T = object> extends ListBoxItemProps<T> {}

export const SelectItem = <T extends object>(props: ListBoxItemProps<T>) => (
  <ListBoxItem {...props} {...composeTextValueProps(props)}>
    {RAC.composeRenderProps(props.children, (children) => (
      <>
        {children}
        <div className={tw`flex-1`} />
        <FiCheckCircle className={tw`hidden size-3.5 stroke-[1.2px] text-success group-selected/list-item:block`} />
      </>
    ))}
  </ListBoxItem>
);
