import { Struct } from 'effect';
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  type MenuItemProps as AriaMenuItemProps,
  type MenuProps as AriaMenuProps,
} from 'react-aria-components';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { DropdownPopover, DropdownPopoverProps } from './dropdown';
import { listBoxItemStyles, listBoxItemVariantKeys, ListBoxItemVariants, listBoxStyles } from './list-box';
import { composeRenderPropsTV } from './utils';

// Root

export interface MenuProps<T extends object>
  extends Omit<AriaMenuProps<T>, 'children'>,
    MixinProps<'popover', Omit<DropdownPopoverProps, 'children'>> {
  children?: AriaMenuProps<T>['children'];
}

export const Menu = <T extends object>({ className, ...props }: MenuProps<T>) => {
  const forwardedProps = splitProps(props, 'popover');

  return (
    <DropdownPopover {...forwardedProps.popover}>
      <AriaMenu {...forwardedProps.rest} className={listBoxStyles({ className })} />
    </DropdownPopover>
  );
};

// Item

export interface MenuItemProps extends AriaMenuItemProps, ListBoxItemVariants {}

export const MenuItem = ({ className, ...props }: MenuItemProps) => {
  const forwardedProps = Struct.omit(props, ...listBoxItemVariantKeys);
  const variantProps = Struct.pick(props, ...listBoxItemVariantKeys);

  return (
    <AriaMenuItem {...forwardedProps} className={composeRenderPropsTV(className, listBoxItemStyles, variantProps)} />
  );
};
