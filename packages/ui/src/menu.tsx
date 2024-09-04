import { Struct } from 'effect';
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  type MenuItemProps as AriaMenuItemProps,
  type MenuProps as AriaMenuProps,
} from 'react-aria-components';
import { type VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { dropdownItemStyles, dropdownListBoxStyles, DropdownPopover, DropdownPopoverProps } from './dropdown';
import { composeRenderPropsTV } from './utils';

// Root

export interface MenuRootProps<T extends object> extends AriaMenuProps<T>, VariantProps<typeof dropdownListBoxStyles> {}

export const MenuRoot = <T extends object>({ className, ...props }: MenuRootProps<T>) => {
  const forwardedProps = Struct.omit(props, ...dropdownListBoxStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownListBoxStyles.variantKeys);
  return <AriaMenu {...forwardedProps} className={dropdownListBoxStyles({ ...variantProps, className })} />;
};

// Root mix

export interface MenuProps<T extends object>
  extends Omit<DropdownPopoverProps, 'children'>,
    MixinProps<'root', Omit<MenuRootProps<T>, 'children'>> {
  children?: MenuRootProps<T>['children'];
}

export const Menu = <T extends object>({ children, ...props }: MenuProps<T>) => {
  const forwardedProps = splitProps(props, 'root');
  return (
    <DropdownPopover {...forwardedProps.rest}>
      <MenuRoot {...forwardedProps.root}>{children}</MenuRoot>
    </DropdownPopover>
  );
};

// Item

export interface MenuItemProps extends AriaMenuItemProps, VariantProps<typeof dropdownItemStyles> {}

export const MenuItem = ({ className, ...props }: MenuItemProps) => {
  const forwardedProps = Struct.omit(props, ...dropdownItemStyles.variantKeys);
  const variantProps = Struct.pick(props, ...dropdownItemStyles.variantKeys);
  return (
    <AriaMenuItem
      {...forwardedProps}
      isDisabled={props.isDisabled ?? false}
      className={composeRenderPropsTV(className, dropdownItemStyles, variantProps)}
    />
  );
};
