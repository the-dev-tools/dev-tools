import { Struct } from 'effect';
import {
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  Popover as AriaPopover,
  composeRenderProps,
  type ListBoxItemProps as AriaListBoxItemProps,
  type ListBoxProps as AriaListBoxProps,
  type PopoverProps as AriaPopoverProps,
} from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv, type VariantProps } from 'tailwind-variants';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Popover

export interface DropdownPopoverProps extends AriaPopoverProps {}

export const DropdownPopover = ({ className, ...props }: DropdownPopoverProps) => (
  <AriaPopover
    {...props}
    className={composeRenderProps(className, (className, { placement }) =>
      twMerge(
        className,
        tw`pointer-events-none flex h-full min-w-[--trigger-width] flex-col`,
        placement === 'top' && tw`flex-col-reverse`,
      ),
    )}
  />
);

// List box

export const dropdownListBoxStyles = tv({
  base: tw`pointer-events-auto flex max-h-full flex-col gap-2 overflow-auto rounded border border-black bg-white p-2 outline-none`,
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
    variant: {
      danger: tw`text-red-600`,
    },
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
