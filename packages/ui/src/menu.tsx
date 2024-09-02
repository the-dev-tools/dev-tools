import { Struct } from 'effect';
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  Popover as AriaPopover,
  PopoverProps,
  type MenuItemProps as AriaMenuItemProps,
  type MenuProps as AriaMenuProps,
  type PopoverProps as AriaPopoverProps,
} from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv, type VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// -----------------------------------------------------------------------------
// Compound Components
// -----------------------------------------------------------------------------

// Popover

export interface MenuPopoverProps extends AriaPopoverProps {}

export const MenuPopover = ({ className, ...props }: MenuPopoverProps) => (
  <AriaPopover {...props} className={composeRenderPropsTW(className, tw`min-w-[--trigger-width]`)} />
);

// Root

export interface MenuRootProps<T extends object> extends AriaMenuProps<T> {}

export const MenuRoot = <T extends object>({ className, ...props }: MenuRootProps<T>) => (
  <AriaMenu
    {...props}
    className={twMerge(tw`flex flex-col gap-2 rounded border border-black bg-white p-2 outline-none`, className)}
  />
);

// Item

export const menuItemStyles = tv({
  extend: focusRingStyles,
  base: tw`select-none rounded px-3 py-2 text-sm leading-none rac-focus:bg-neutral-400`,
  variants: {
    isDisabled: { false: tw`cursor-pointer` },
  },
});

export interface MenuItemProps extends AriaMenuItemProps, VariantProps<typeof menuItemStyles> {}

export const MenuItem = ({ className, ...props }: MenuItemProps) => {
  const forwardedProps = Struct.omit(props, ...menuItemStyles.variantKeys);
  const variantProps = Struct.pick(props, ...menuItemStyles.variantKeys);
  return (
    <AriaMenuItem
      {...forwardedProps}
      isDisabled={props.isDisabled ?? false}
      className={composeRenderPropsTV(className, menuItemStyles, variantProps)}
    />
  );
};

// -----------------------------------------------------------------------------
// Mix Components
// -----------------------------------------------------------------------------

export interface MenuProps<T extends object>
  extends Omit<PopoverProps, 'children'>,
    MixinProps<'root', Omit<MenuRootProps<T>, 'children'>> {
  children?: MenuRootProps<T>['children'];
}

export const Menu = <T extends object>({ children, ...props }: MenuProps<T>) => {
  const forwardedProps = splitProps(props, 'root');
  return (
    <MenuPopover {...forwardedProps.rest}>
      <MenuRoot {...forwardedProps.root}>{children}</MenuRoot>
    </MenuPopover>
  );
};
