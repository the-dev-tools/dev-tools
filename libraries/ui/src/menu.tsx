import { Struct } from 'effect';
import { RefObject, useCallback, useRef, useState } from 'react';
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  type MenuItemProps as AriaMenuItemProps,
  type MenuProps as AriaMenuProps,
} from 'react-aria-components';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { DropdownPopover, DropdownPopoverProps } from './dropdown';
import { listBoxItemStyles, listBoxItemVariantKeys, ListBoxItemVariants, listBoxStyles } from './list-box';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Root

export interface MenuProps<T extends object>
  extends Omit<AriaMenuProps<T>, 'children'>,
    MixinProps<'popover', Omit<DropdownPopoverProps, 'children'>> {
  children?: AriaMenuProps<T>['children'];
  contextMenuRef?: RefObject<HTMLDivElement | null>;
  contextMenuPosition?: ContextMenuPosition | undefined;
}

export const Menu = <T extends object>({
  className,
  contextMenuRef,
  contextMenuPosition,
  popoverTriggerRef,
  ...props
}: MenuProps<T>) => {
  const forwardedProps = splitProps(props, 'popover');

  const triggerRef = contextMenuPosition ? contextMenuRef : popoverTriggerRef;

  return (
    <>
      {contextMenuRef && <div ref={contextMenuRef} className={tw`fixed`} style={contextMenuPosition} />}

      <DropdownPopover triggerRef={triggerRef!} {...forwardedProps.popover}>
        <AriaMenu {...forwardedProps.rest} className={listBoxStyles({ className })} />
      </DropdownPopover>
    </>
  );
};

// Context menu state

interface ContextMenuPosition {
  top: number;
  left: number;
}

export const useContextMenuState = () => {
  const contextMenuRef = useRef<HTMLDivElement>(null);

  const [{ isOpen, contextMenuPosition }, setState] = useState<{
    isOpen: boolean;
    contextMenuPosition?: ContextMenuPosition;
  }>({ isOpen: false });

  const onContextMenu = useCallback((event: React.MouseEvent, offset?: ContextMenuPosition) => {
    setState({
      isOpen: true,
      contextMenuPosition: {
        left: event.pageX - (offset?.left ?? 0),
        top: event.pageY - (offset?.top ?? 0),
      },
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
