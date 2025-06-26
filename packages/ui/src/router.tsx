import { Rx, useRxSet, useRxValue } from '@effect-rx/rx-react';
import {
  ActiveLinkOptions,
  LinkComponent as LinkComponentUpstream,
  MatchRouteOptions,
  ToOptions,
  useLinkProps,
  useRouter,
} from '@tanstack/react-router';
import { Array, Option, pipe } from 'effect';
import React, { ComponentProps, PropsWithChildren, ReactNode, Ref, Suspense, SyntheticEvent } from 'react';
import { ListBox, ListBoxItem, RouterProvider } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import { Button } from './button';
import { tw } from './tailwind-literal';

declare module 'react-aria-components' {
  interface RouterConfig {
    href: unknown;
    routerOptions: unknown;
  }
}

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

export interface Tab {
  baseRoute: ToOptions;
  route: ToOptions;

  matchOptions?: MatchRouteOptions | undefined;
  node: ReactNode;
}

export interface TabProps {
  tab?: ReactNode;
  tabBaseRoute?: ToOptions;
  tabMatchOptions?: MatchRouteOptions;
}

const tabsRx = pipe(Array.empty<Tab>(), (_) => Rx.make(_), Rx.keepAlive);

const useTabKey = () => {
  const router = useRouter();
  return (tab: Pick<Tab, 'baseRoute'>) => router.buildLocation(tab.baseRoute).href;
};

export interface UseLinkProps extends ActiveLinkOptions, TabProps {
  children?: ((state: { isActive: boolean; isTransitioning: boolean }) => React.ReactNode) | React.ReactNode;
  ref?: Ref<unknown> | undefined;
}

export const useLink = ({ children, ref, tab: tabNode, tabBaseRoute, tabMatchOptions, ...props }: UseLinkProps) => {
  const router = useRouter();
  const setTabs = useRxSet(tabsRx);

  const _ = useLinkProps(props, ref as Ref<Element>) as ComponentProps<'a'> & Record<string, unknown>;

  const isActive = _['data-status'] === 'active';
  const isTransitioning = _['data-transitioning'] === 'transitioning';

  const onActionBase = fauxEvent(_.onClick, { button: 0 });
  const onActionTab: typeof onActionBase = (event) => {
    const tab: Tab = {
      baseRoute: tabBaseRoute ?? props,
      matchOptions: tabMatchOptions,
      node: tabNode,
      route: props,
    };

    const updateTabs = (tabs: Tab[]) =>
      pipe(
        Array.findFirstIndex(tabs, (_) => router.matchRoute(_.baseRoute, _.matchOptions) !== false),
        Option.flatMap((_) => Array.modifyOption(tabs, _, () => tab)),
        Option.getOrElse(() => Array.append(tabs, tab)),
      );

    onActionBase(event);
    setTimeout(() => void setTabs(updateTabs), 0);
  };

  return {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ref: _.ref as Ref<any>,

    children: typeof children === 'function' ? children({ isActive, isTransitioning }) : children,
    href: _.href!,

    isActive,
    isDisabled: _['disabled'] === true,
    isTransitioning,

    onAction: tabNode ? onActionTab : onActionBase,
    onFocus: fauxEvent(_.onFocus),
    onHoverEnd: fauxEvent(_.onMouseLeave),
    onHoverStart: fauxEvent(_.onMouseEnter),
  };
};

export type LinkComponent<T = object> = LinkComponentUpstream<(props: T & TabProps) => ReactNode>;

interface TabItemProps extends ToOptions {
  id: string;
  tab: Tab;
}

const TabItem = ({ id, tab }: TabItemProps) => {
  const setTabs = useRxSet(tabsRx);
  const { isActive, ...linkProps } = useLink({ ...tab.route, activeOptions: { exact: true } });

  return (
    <ListBoxItem
      aria-label='Tab'
      className={twMerge(
        tw`
          relative -ml-px flex h-11 max-w-60 cursor-pointer items-center justify-between gap-3 border p-2.5 text-xs
          leading-4 font-medium tracking-tight text-slate-800

          before:absolute before:-left-px before:h-6 before:w-px before:bg-gray-200
        `,
        !isActive && tw`border-b border-transparent border-b-gray-200 opacity-60`,
        isActive && tw`rounded-t-md border border-gray-200 border-b-transparent bg-white`,
      )}
      id={id}
      {...linkProps}
    >
      <div className={tw`flex min-w-0 flex-1 items-center gap-1.5`}>
        <Suspense fallback='Loading...'>{tab.node}</Suspense>
      </div>

      <Button
        className={tw`p-0.5`}
        onPress={(event) => {
          event.continuePropagation();
          void setTabs(Array.filter((_) => _ !== tab));
        }}
        variant='ghost'
      >
        <FiX className={tw`size-4 text-slate-500`} />
      </Button>
    </ListBoxItem>
  );
};

export const RouteTabList = () => {
  const tabs = useRxValue(tabsRx);
  const tabKey = useTabKey();

  return (
    <ListBox
      aria-label='Tabs'
      className={tw`
        relative flex h-11 w-full overflow-auto

        before:absolute before:bottom-0 before:w-full before:border-b before:border-gray-200
      `}
      items={tabs}
      orientation='horizontal'
      selectionMode='none'
    >
      {(_) => <TabItem id={tabKey(_)} tab={_} />}
    </ListBox>
  );
};
