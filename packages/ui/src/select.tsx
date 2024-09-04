import { Struct } from 'effect';
import {
  Button as AriaButton,
  Select as AriaSelect,
  SelectValue as AriaSelectValue,
  type ButtonProps as AriaButtonProps,
  type SelectProps as AriaSelectProps,
  type SelectValueProps as AriaSelectValueProps,
} from 'react-aria-components';
import { IconBaseProps } from 'react-icons';
import { LuChevronDown } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { tv, type VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { buttonStyles } from './button';
import { DropdownListBox, DropdownListBoxProps, DropdownPopover, DropdownPopoverProps } from './dropdown';
import { FieldError, FieldLabel, type FieldErrorProps, type FieldLabelProps } from './field';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Root

export interface SelectRootProps<T extends object> extends AriaSelectProps<T> {}

export const SelectRoot = <T extends object>({ className, ...props }: SelectRootProps<T>) => (
  <AriaSelect {...props} className={composeRenderPropsTW(className, tw`flex flex-col gap-1`)} />
);

// Trigger

export const selectTriggerStyles = tv({
  extend: buttonStyles,
  defaultVariants: {
    kind: 'placeholder',
    variant: 'placeholder',
  },
});

export interface SelectTriggerProps extends AriaButtonProps, VariantProps<typeof selectTriggerStyles> {}

export const SelectTrigger = ({ className, ...props }: SelectTriggerProps) => {
  const forwardedProps = Struct.omit(props, ...selectTriggerStyles.variantKeys);
  const variantProps = Struct.pick(props, ...selectTriggerStyles.variantKeys);
  return (
    <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, selectTriggerStyles, variantProps)} />
  );
};

// Indicator

export interface SelectIndicatorProps extends IconBaseProps {
  isOpen: boolean;
}

export const SelectIndicator = ({ isOpen, ...props }: SelectIndicatorProps) => (
  <LuChevronDown {...props} className={twJoin(tw`size-4 transition-transform`, isOpen && tw`rotate-180`)} />
);

// Mix

export interface SelectProps<T extends object>
  extends Omit<SelectRootProps<T>, 'children'>,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'trigger', Omit<SelectTriggerProps, 'children'>>,
    MixinProps<'value', Omit<AriaSelectValueProps<T>, 'children'>>,
    MixinProps<'indicator', Omit<SelectIndicatorProps, 'children' | 'isOpen'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>>,
    MixinProps<'popover', Omit<DropdownPopoverProps, 'children'>>,
    MixinProps<'listBox', Omit<DropdownListBoxProps<T>, 'children'>> {
  children?: DropdownListBoxProps<T>['children'];
  label?: FieldLabelProps['children'];
  error?: FieldErrorProps['children'];
}

export const Select = <T extends object>({ children, label, error, ...props }: SelectProps<T>) => {
  const forwardedProps = splitProps(props, 'label', 'trigger', 'value', 'indicator', 'error', 'popover', 'listBox');
  return (
    <SelectRoot {...forwardedProps.rest}>
      {({ isOpen }) => (
        <>
          {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
          <SelectTrigger {...forwardedProps.trigger}>
            <AriaSelectValue {...forwardedProps.value} />
            <SelectIndicator {...forwardedProps.indicator} isOpen={isOpen} />
          </SelectTrigger>
          <FieldError {...forwardedProps.error}>{error}</FieldError>
          <DropdownPopover {...forwardedProps.popover}>
            <DropdownListBox>{children}</DropdownListBox>
          </DropdownPopover>
        </>
      )}
    </SelectRoot>
  );
};
