import { Struct } from 'effect';
import { Dialog as AriaDialog, type DialogProps as AriaDialogProps } from 'react-aria-components';
import { VariantProps } from 'tailwind-variants';

import { dropdownListBoxStyles, DropdownPopover, DropdownPopoverProps } from './dropdown';
import { MixinProps, splitProps } from './mixin-props';

// Dialog

export interface PopoverDialogProps extends AriaDialogProps, VariantProps<typeof dropdownListBoxStyles> {}

export const PopoverDialog = ({ className, ...props }: PopoverDialogProps) => {
  const forwardedProps = Struct.omit(props, ...dropdownListBoxStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownListBoxStyles.variantKeys);
  return <AriaDialog {...forwardedProps} className={dropdownListBoxStyles({ ...variantProps, className })} />;
};

// Mix

export interface PopoverProps
  extends MixinProps<'dialog', Omit<PopoverDialogProps, 'children'>>,
    Omit<DropdownPopoverProps, 'children'> {
  children?: PopoverDialogProps['children'];
}

export const Popover = ({ children, ...props }: PopoverProps) => {
  const forwardedProps = splitProps(props, 'dialog');
  return (
    <DropdownPopover {...forwardedProps.rest}>
      <PopoverDialog {...forwardedProps.dialog}>{children}</PopoverDialog>
    </DropdownPopover>
  );
};
