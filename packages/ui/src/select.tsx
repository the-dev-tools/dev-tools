import { Struct } from 'effect';
import { RefAttributes } from 'react';
import { mergeProps } from 'react-aria';
import * as RAC from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { FiCheckCircle, FiChevronDown } from 'react-icons/fi';
import { Button, ButtonProps } from './button';
import { FieldError, type FieldErrorProps, FieldLabel, type FieldLabelProps } from './field';
import { ListBox, ListBoxItem, ListBoxItemProps, ListBoxProps } from './list-box';
import { Popover } from './popover';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps, composeTextValueProps } from './utils';

// Root

export interface SelectProps<T extends object>
  extends Omit<RAC.SelectProps<T>, 'children'>,
    Pick<ListBoxProps<T>, 'children' | 'items'>,
    RefAttributes<HTMLDivElement> {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
  triggerClassName?: ButtonProps['className'];
  triggerVariant?: ButtonProps['variant'];
  value?: RAC.SelectValueProps<T>['children'];
}

export const Select = <T extends object>({
  children,
  className,
  error,
  items,
  label,
  triggerClassName,
  triggerVariant,
  value,
  ...props
}: SelectProps<T>) => (
  <RAC.Select {...props} className={composeTailwindRenderProps(className, tw`group flex flex-col gap-1`)}>
    {label && <FieldLabel>{label}</FieldLabel>}
    <Button className={triggerClassName!} variant={triggerVariant}>
      <RAC.SelectValue>{value}</RAC.SelectValue>
      <FiChevronDown className={tw`-mr-1 size-4 text-slate-500 transition-transform group-open:rotate-180`} />
    </Button>
    {error && <FieldError>{error}</FieldError>}
    <Popover>
      <ListBox items={items!}>{children}</ListBox>
    </Popover>
  </RAC.Select>
);

// Item

export interface SelectItemProps extends ListBoxItemProps {}

export const SelectItem = (props: ListBoxItemProps) => (
  <ListBoxItem {...props} {...composeTextValueProps(props)}>
    {RAC.composeRenderProps(props.children, (children) => (
      <>
        {children}
        <div className={tw`flex-1`} />
        <FiCheckCircle className={tw`hidden size-3.5 stroke-[1.2px] text-green-600 group-selected/list-item:block`} />
      </>
    ))}
  </ListBoxItem>
);

// RHF wrapper

export interface SelectRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<SelectProps<TFieldValues>, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const SelectRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: SelectRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const {
    field: { ref, ...field },
    fieldState,
  } = useController({ defaultValue: null as never, ...controllerProps });

  const fieldProps: SelectProps<TFieldValues> = {
    error: fieldState.error?.message,
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    name: field.name,
    onBlur: field.onBlur,
    onSelectionChange: field.onChange,
    selectedKey: field.value,
    validationBehavior: 'aria',
  };

  return <Select {...mergeProps(fieldProps, forwardedProps)} ref={ref} />;
};
