import { Registry, Rx, useRxSet, useRxValue } from '@effect-rx/rx-react';
import {
  ActiveLinkOptions,
  AnyRouteMatch,
  LinkComponent as LinkComponentUpstream,
  ToOptions,
  useLinkProps,
} from '@tanstack/react-router';
import { Array, Option, pipe, Runtime } from 'effect';
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
  id: string;
  node: ReactNode;
  route: ToOptions;
}

export type TabsRx = ReturnType<typeof makeTabsRx>;

export const makeTabsRx = () => pipe(Array.empty<Tab>(), (_) => Rx.make(_), Rx.keepAlive);

interface AddTabProps {
  match: Omit<AnyRouteMatch, 'context'> & {
    context: {
      runtime: Runtime.Runtime<Registry.RxRegistry>;
      tabsRx: TabsRx;
    };
  };
  node: ReactNode;
}

export const addTab = ({ match, node }: AddTabProps) => {
  const { runtime, tabsRx } = match.context;
  const tab: Tab = {
    id: match.id,
    node,
    route: {
      from: '/',
      params: match.params,
      search: match.search as unknown,
      to: match.fullPath as unknown,
    },
  };

  const updateTabs = (tabs: Tab[]) =>
    pipe(
      Array.findFirstIndex(tabs, (_) => _.id === tab.id),
      Option.flatMap((_) => Array.modifyOption(tabs, _, () => tab)),
      Option.getOrElse(() => Array.append(tabs, tab)),
    );

  pipe(Rx.update(tabsRx, updateTabs), Runtime.runSync(runtime));
};

export interface UseLinkProps extends ActiveLinkOptions {
  children?: ((state: { isActive: boolean; isTransitioning: boolean }) => React.ReactNode) | React.ReactNode;
  ref?: Ref<unknown> | undefined;
}

export const useLink = ({ children, ref, ...props }: UseLinkProps) => {
  const _ = useLinkProps(props, ref as Ref<Element>) as ComponentProps<'a'> & Record<string, unknown>;

  const isActive = _['data-status'] === 'active';
  const isTransitioning = _['data-transitioning'] === 'transitioning';

  return {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ref: _.ref as Ref<any>,

    children: typeof children === 'function' ? children({ isActive, isTransitioning }) : children,
    href: _.href!,

    isActive,
    isDisabled: _['disabled'] === true,
    isTransitioning,

    onAction: fauxEvent(_.onClick, { button: 0 }),
    onFocus: fauxEvent(_.onFocus),
    onHoverEnd: fauxEvent(_.onMouseLeave),
    onHoverStart: fauxEvent(_.onMouseEnter),
  };
};

export type LinkComponent<T = object> = LinkComponentUpstream<(props: T) => ReactNode>;

interface TabItemProps extends ToOptions {
  id: string;
  tab: Tab;
  tabsRx: TabsRx;
}

const TabItem = ({ id, tab, tabsRx }: TabItemProps) => {
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

interface RouteTabListProps {
  tabsRx: TabsRx;
}

export const RouteTabList = ({ tabsRx }: RouteTabListProps) => {
  const tabs = useRxValue(tabsRx);

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
      {(_) => <TabItem id={_.id} tab={_} tabsRx={tabsRx} />}
    </ListBox>
  );
};
