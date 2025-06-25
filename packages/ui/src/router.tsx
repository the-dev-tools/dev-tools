import { ActiveLinkOptions, LinkComponent as LinkComponentUpstream, useLinkProps } from '@tanstack/react-router';
import React, { ComponentProps, PropsWithChildren, ReactNode, Ref, SyntheticEvent } from 'react';
import { RouterProvider } from 'react-aria-components';

export const AriaRouterProvider = ({ children }: PropsWithChildren) => (
  <RouterProvider navigate={() => undefined}>{children}</RouterProvider>
);

const fauxEvent =
  <E extends SyntheticEvent>(handler: React.EventHandler<E> | undefined, defaultEvent?: Partial<E>) =>
  (event?: object) =>
    handler?.({
      defaultPrevented: false,
      preventDefault: () => undefined,
      ...defaultEvent,
      ...event,
    } as E);

export interface UseLinkProps extends ActiveLinkOptions {
  children?: ((state: { isActive: boolean; isTransitioning: boolean }) => React.ReactNode) | React.ReactNode;
  ref?: Ref<unknown> | undefined;
}

export const useLink = ({ children, ref, ...props }: UseLinkProps) => {
  const _ = useLinkProps(props, ref as Ref<Element>) as ComponentProps<'a'> & Record<string, unknown>;

  const isActive = _['data-status'] === 'active';
  const isTransitioning = _['data-transitioning'] === 'transitioning';

  const onAction = fauxEvent(_.onClick, { button: 0 });

  return {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ref: _.ref as Ref<any>,

    children: typeof children === 'function' ? children({ isActive, isTransitioning }) : children,
    href: _.href!,

    isActive,
    isDisabled: _['disabled'] === true,
    isTransitioning,

    onAction,
    onFocus: fauxEvent(_.onFocus),
    onHoverEnd: fauxEvent(_.onMouseLeave),
    onHoverStart: fauxEvent(_.onMouseEnter),
  };
};

export type LinkComponent<T = object> = LinkComponentUpstream<(props: T) => ReactNode>;
