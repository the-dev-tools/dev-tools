import { Struct } from 'effect';
import {
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  type ListBoxItemProps as AriaListBoxItemProps,
  type ListBoxProps as AriaListBoxProps,
  Popover as AriaPopover,
  type PopoverProps as AriaPopoverProps,
  composeRenderProps,
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
        tw`pointer-events-none flex h-full min-w-(--trigger-width) flex-col`,
        placement === 'top' && tw`flex-col-reverse`,
      ),
    )}
  />
);

// List box

export const dropdownListBoxStyles = tv({
  base: tw`
    pointer-events-auto flex max-h-full flex-col gap-2 overflow-auto rounded-sm border border-black bg-white p-2
    outline-hidden
  `,
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
  base: tw`rounded-sm px-3 py-2 text-sm leading-none select-none rac-focus:bg-neutral-400`,
  variants: {
    isDisabled: { false: tw`cursor-pointer` },
    variant: {
      danger: tw`text-red-600`,
    },
  },
});

export interface DropdownItemProps extends AriaListBoxItemProps, VariantProps<typeof dropdownItemStyles> {}

export const DropdownItem = ({ className, ...props }: DropdownItemProps) => {
  const forwardedProps = Struct.omit(props, ...dropdownItemStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownItemStyles.variantKeys);
  return (
    <AriaListBoxItem
      {...forwardedProps}
      className={composeRenderPropsTV(className, dropdownItemStyles, variantProps)}
      isDisabled={props.isDisabled ?? false}
    />
  );
};
