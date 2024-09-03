import { Struct } from 'effect';
import {
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  Popover as AriaPopover,
  type ListBoxItemProps as AriaListBoxItemProps,
  type ListBoxProps as AriaListBoxProps,
  type PopoverProps as AriaPopoverProps,
} from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Popover

export const dropdownPopoverStyles = tv({
  base: tw`min-w-[--trigger-width]`,
});

export interface DropdownPopoverProps extends AriaPopoverProps, VariantProps<typeof dropdownPopoverStyles> {}

export const DropdownPopover = ({ className, ...props }: DropdownPopoverProps) => {
  const forwardedProps = Struct.omit(props, ...dropdownPopoverStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownPopoverStyles.variantKeys);
  return (
    <AriaPopover {...forwardedProps} className={composeRenderPropsTV(className, dropdownPopoverStyles, variantProps)} />
  );
};

// List box

export const dropdownListBoxStyles = tv({
  base: tw`flex flex-col gap-2 rounded border border-black bg-white p-2 outline-none`,
});

export interface DropdownListBoxProps<T extends object>
  extends AriaListBoxProps<T>,
    VariantProps<typeof dropdownListBoxStyles> {}

export const DropdownListBox = <T extends object>({ className, ...props }: DropdownListBoxProps<T>) => {
  const forwardedProps = Struct.omit(props, ...dropdownListBoxStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownListBoxStyles.variantKeys);
  return (
    <AriaListBox {...forwardedProps} className={composeRenderPropsTV(className, dropdownListBoxStyles, variantProps)} />
  );
};

// Item

export const dropdownItemStyles = tv({
  extend: focusRingStyles,
  base: tw`select-none rounded px-3 py-2 text-sm leading-none rac-focus:bg-neutral-400`,
  variants: {
    isDisabled: { false: tw`cursor-pointer` },
  },
});

export interface DropdownItemProps extends AriaListBoxItemProps, VariantProps<typeof dropdownItemStyles> {}

export const DropdownItem = ({ className, ...props }: DropdownItemProps) => {
  const forwardedProps = Struct.omit(props, ...dropdownItemStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownItemStyles.variantKeys);
  return (
    <AriaListBoxItem
      {...forwardedProps}
      isDisabled={props.isDisabled ?? false}
      className={composeRenderPropsTV(className, dropdownItemStyles, variantProps)}
    />
  );
};
