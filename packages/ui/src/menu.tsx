import { Struct } from 'effect';
import { RefObject, useCallback, useRef, useState } from 'react';
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  type MenuItemProps as AriaMenuItemProps,
  type MenuProps as AriaMenuProps,
} from 'react-aria-components';
import { DropdownPopover, DropdownPopoverProps } from './dropdown';
import { listBoxItemStyles, listBoxItemVariantKeys, ListBoxItemVariants, listBoxStyles } from './list-box';
import { type MixinProps, splitProps } from './mixin-props';
import { LinkComponent, useLink, UseLinkProps } from './router';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Root

export interface MenuProps<T extends object>
  extends MixinProps<'popover', Omit<DropdownPopoverProps, 'children'>>,
    Omit<AriaMenuProps<T>, 'children'> {
  children?: AriaMenuProps<T>['children'];
  contextMenuPosition?: ContextMenuPosition | undefined;
  contextMenuRef?: RefObject<HTMLDivElement | null>;
}

export const Menu = <T extends object>({
  className,
  contextMenuPosition,
  contextMenuRef,
  popoverTriggerRef,
  ...props
}: MenuProps<T>) => {
  const forwardedProps = splitProps(props, 'popover');

  const triggerRef = contextMenuPosition ? contextMenuRef : popoverTriggerRef;

  return (
    <>
      {contextMenuRef && <div className={tw`fixed`} ref={contextMenuRef} style={contextMenuPosition} />}

      <DropdownPopover triggerRef={triggerRef!} {...forwardedProps.popover}>
        <AriaMenu {...forwardedProps.rest} className={composeRenderPropsTV(className, listBoxStyles)} />
      </DropdownPopover>
    </>
  );
};

// Context menu state

interface ContextMenuPosition {
  left: number;
  top: number;
}

export const useContextMenuState = () => {
  const contextMenuRef = useRef<HTMLDivElement>(null);

  const [{ contextMenuPosition, isOpen }, setState] = useState<{
    contextMenuPosition?: ContextMenuPosition;
    isOpen: boolean;
  }>({ isOpen: false });

  const onContextMenu = useCallback((event: React.MouseEvent, offset?: ContextMenuPosition, zoom = 1) => {
    setState({
      contextMenuPosition: {
        left: (event.pageX - (offset?.left ?? 0)) / zoom,
        top: (event.pageY - (offset?.top ?? 0)) / zoom,
      },
      isOpen: true,
    });

    event.preventDefault();
  }, []);

  const onOpenChange = useCallback((isOpen: boolean) => void setState({ isOpen }), []);

  return {
    menuProps: { contextMenuPosition, contextMenuRef },
    menuTriggerProps: { isOpen, onOpenChange },
    onContextMenu,
  };
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

export const MenuItemLink: LinkComponent<MenuItemProps> = (props) => {
  const linkProps = useLink(props as UseLinkProps);
  return <MenuItem {...(props as MenuItemProps)} {...linkProps} />;
};
