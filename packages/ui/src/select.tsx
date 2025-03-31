import { Struct } from 'effect';
import { FC, ForwardedRef, forwardRef, RefAttributes } from 'react';
import { mergeProps } from 'react-aria';
import {
  Button as AriaButton,
  type ButtonProps as AriaButtonProps,
  Select as AriaSelect,
  type SelectProps as AriaSelectProps,
  SelectValue as AriaSelectValue,
  type SelectValueProps as AriaSelectValueProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { IconBaseProps } from 'react-icons';
import { FiChevronDown } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { type VariantProps } from 'tailwind-variants';

import { buttonStyles } from './button';
import { DropdownPopover, DropdownPopoverProps } from './dropdown';
import { FieldError, type FieldErrorProps, FieldLabel, type FieldLabelProps } from './field';
import { ListBox, ListBoxProps } from './list-box';
import { type MixinProps, splitProps } from './mixin-props';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Root

export interface SelectRootProps<T extends object> extends AriaSelectProps<T> {}

export const SelectRoot = <T extends object>({ className, ...props }: SelectRootProps<T>) => (
  <AriaSelect {...props} className={composeRenderPropsTW(className, tw`flex flex-col gap-1`)} />
);

// Trigger

export interface SelectTriggerProps extends AriaButtonProps, VariantProps<typeof buttonStyles> {}

export const SelectTrigger = forwardRef(
  ({ className, ...props }: SelectTriggerProps, ref: ForwardedRef<HTMLButtonElement>) => {
    const forwardedProps = Struct.omit(props, ...buttonStyles.variantKeys);
    const variantProps = Struct.pick(props, ...buttonStyles.variantKeys);
    return (
      <AriaButton
        {...forwardedProps}
        className={composeRenderPropsTV(className, buttonStyles, variantProps)}
        ref={ref}
      />
    );
  },
);
SelectTrigger.displayName = 'SelectTrigger';

// Indicator

export interface SelectIndicatorProps extends IconBaseProps {
  isOpen: boolean;
}

export const SelectIndicator = ({ isOpen, ...props }: SelectIndicatorProps) => (
  <FiChevronDown
    {...props}
    className={twJoin(tw`-mr-1 size-4 text-slate-500 transition-transform`, isOpen && tw`rotate-180`)}
  />
);

// Mix

export interface SelectProps<T extends object>
  extends MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'trigger', Omit<SelectTriggerProps, 'children'>>,
    MixinProps<'value', Omit<AriaSelectValueProps<T>, 'children'>>,
    MixinProps<'indicator', Omit<SelectIndicatorProps, 'children' | 'isOpen'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>>,
    MixinProps<'popover', Omit<DropdownPopoverProps, 'children'>>,
    MixinProps<'listBox', Omit<ListBoxProps<T>, 'children'>>,
    Omit<SelectRootProps<T>, 'children'>,
    RefAttributes<HTMLButtonElement> {
  children?: ListBoxProps<T>['children'];
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
  value?: AriaSelectValueProps<T>['children'];
}

interface Select extends FC<SelectProps<object>> {
  <T extends object>(props: SelectProps<T>): ReturnType<FC<SelectProps<T>>>;
}

export const Select: Select = forwardRef(({ children, error, label, value, ...props }, ref) => {
  const forwardedProps = splitProps(props, 'label', 'trigger', 'value', 'indicator', 'error', 'popover', 'listBox');
  return (
    <SelectRoot {...forwardedProps.rest}>
      {({ isOpen }) => (
        <>
          {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
          <SelectTrigger {...forwardedProps.trigger} ref={ref}>
            <AriaSelectValue {...forwardedProps.value}>{value}</AriaSelectValue>
            <SelectIndicator {...forwardedProps.indicator} isOpen={isOpen} />
          </SelectTrigger>
          <FieldError {...forwardedProps.error}>{error}</FieldError>
          <DropdownPopover {...forwardedProps.popover}>
            <ListBox {...forwardedProps.listBox}>{children}</ListBox>
          </DropdownPopover>
        </>
      )}
    </SelectRoot>
  );
});
Select.displayName = 'Select';

// RHF wrapper mix

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

  const { field, fieldState } = useController({ defaultValue: null as never, ...controllerProps });

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

  return <Select {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
