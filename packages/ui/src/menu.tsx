import { RefObject, useCallback, useRef, useState } from 'react';
import * as RAC from 'react-aria-components';
import { FiChevronRight } from 'react-icons/fi';
import { tv, VariantProps } from 'tailwind-variants';
import { listBoxItemStyles, listBoxStyles } from './list-box';
import { Popover } from './popover';
import { LinkComponent, useLink, UseLinkProps } from './router';
import { tw } from './tailwind-literal';
import { composeStyleProps, composeStyleRenderProps } from './utils';

// Root

export const menuStyles = tv({ extend: listBoxStyles });

export interface MenuProps<T extends object> extends ContextMenuProps, RAC.MenuProps<T> {}

export const Menu = <T extends object>({ className, contextMenuPosition, contextMenuRef, ...props }: MenuProps<T>) => (
  <>
    {contextMenuRef && <div className={tw`fixed`} ref={contextMenuRef} style={contextMenuPosition} />}

    <Popover
      className={tw`data-[trigger=SubmenuTrigger]:placement-right:-ml-1`}
      {...(contextMenuPosition && { triggerRef: contextMenuRef })}
    >
      <RAC.Menu {...props} className={composeStyleRenderProps(className, menuStyles)} />
    </Popover>
  </>
);

// Context menu state

export interface ContextMenuPosition {
  left: number;
  top: number;
}

export interface ContextMenuProps {
  contextMenuPosition?: ContextMenuPosition | undefined;
  contextMenuRef?: RefObject<HTMLDivElement | null>;
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
    menuProps: { contextMenuPosition, contextMenuRef } satisfies ContextMenuProps,
    menuTriggerProps: { isOpen, onOpenChange } satisfies Partial<RAC.MenuTriggerProps>,
    onContextMenu,
  };
};

// Item

export const menuItemStyles = tv({ extend: listBoxItemStyles });

export interface MenuItemProps extends RAC.MenuItemProps, VariantProps<typeof menuItemStyles> {}

export const MenuItem = ({ children, ...props }: MenuItemProps) => (
  <RAC.MenuItem {...props} className={composeStyleProps(props, menuItemStyles)}>
    {RAC.composeRenderProps(children, (children, { hasSubmenu }) => (
      <>
        {children}
        <div className={tw`flex-1`} />
        {hasSubmenu && <FiChevronRight className={tw`size-3 text-slate-500`} />}
      </>
    ))}
  </RAC.MenuItem>
);

export const MenuItemLink: LinkComponent<MenuItemProps> = (props) => {
  const linkProps = useLink(props as UseLinkProps);
  return <MenuItem {...(props as MenuItemProps)} {...linkProps} />;
};
