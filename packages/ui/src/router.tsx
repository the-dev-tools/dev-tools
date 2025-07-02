import { Registry, Rx, useRxValue } from '@effect-rx/rx-react';
import {
  ActiveLinkOptions,
  AnyRouteMatch,
  LinkComponent as LinkComponentUpstream,
  ToOptions,
  useLinkProps,
  useRouter,
} from '@tanstack/react-router';
import { Array, Effect, Match, Option, pipe, Runtime } from 'effect';
import React, { ComponentProps, PropsWithChildren, ReactNode, Ref, Suspense, SyntheticEvent, useEffect } from 'react';
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

export interface TabsRouteContext {
  baseRoute: ToOptions;
  runtime: Runtime.Runtime<Registry.RxRegistry>;
  tabsRx: TabsRx;
}

interface AddTabProps {
  id: string;
  match: Omit<AnyRouteMatch, 'context'> & { context: TabsRouteContext };
  node: ReactNode;
}

export const addTab = ({ id, match, node }: AddTabProps) => {
  const { runtime, tabsRx } = match.context;
  const tab: Tab = {
    id,
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

interface RemoveTabProps extends TabsRouteContext {
  id: string;
}

export const useRemoveTab = () => {
  const router = useRouter();

  return async ({ baseRoute, id, runtime, tabsRx }: RemoveTabProps) =>
    Effect.gen(function* () {
      let tabs = yield* Rx.get(tabsRx);

      const index = Array.findFirstIndex(tabs, (_) => _.id === id);
      if (Option.isNone(index)) return;
      const tab = Array.unsafeGet(tabs, index.value);

      tabs = Array.remove(tabs, index.value);
      yield* Rx.set(tabsRx, tabs);

      const match: unknown = router.matchRoute(tab.route);
      if (match === false) return;

      const nextTab = pipe(
        Array.get(tabs, index.value),
        Option.orElse(() => Array.last(tabs)),
      );

      if (Option.isNone(nextTab)) {
        void router.navigate(baseRoute);
      } else {
        void router.navigate(nextTab.value.route);
      }
    }).pipe(Runtime.runPromise(runtime));
};

interface UseTabShortcutsProps extends TabsRouteContext {}

const useTabShortcuts = ({ baseRoute, runtime, tabsRx }: UseTabShortcutsProps) => {
  const router = useRouter();

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) =>
      Effect.gen(function* () {
        const { code, ctrlKey, metaKey, shiftKey } = event;
        let shortcut: 'close' | 'next' | 'prev' | undefined;
        if ((ctrlKey || metaKey) && code === 'Tab') shortcut = 'next';
        if ((ctrlKey || metaKey) && shiftKey && code === 'Tab') shortcut = 'prev';
        if ((ctrlKey || metaKey) && code === 'KeyW') shortcut = 'close';
        if (!shortcut) return;

        event.preventDefault();

        let tabs = yield* Rx.get(tabsRx);
        const index = Array.findFirstIndex(tabs, (_) => router.matchRoute(_.route) !== false);

        if (Option.isNone(index)) return;

        if (shortcut === 'close') {
          tabs = Array.remove(tabs, index.value);
          yield* Rx.set(tabsRx, tabs);
        }

        const tab = pipe(
          Match.value(shortcut),
          Match.when('close', () =>
            pipe(
              Array.get(tabs, index.value),
              Option.orElse(() => Array.last(tabs)),
            ),
          ),
          Match.when('next', () =>
            pipe(
              Array.get(tabs, index.value + 1),
              Option.orElse(() => Array.head(tabs)),
            ),
          ),
          Match.when('prev', () =>
            pipe(
              Array.get(tabs, index.value - 1),
              Option.orElse(() => Array.last(tabs)),
            ),
          ),
          Match.exhaustive,
        );

        if (Option.isNone(tab)) {
          void router.navigate(baseRoute);
        } else {
          void router.navigate(tab.value.route);
        }
      }).pipe(Runtime.runPromise(runtime));

    window.addEventListener('keydown', onKeyDown);
    return () => void window.removeEventListener('keydown', onKeyDown);
  }, [baseRoute, router, runtime, tabsRx]);
};

// https://github.com/facebook/react/issues/29832#issuecomment-2490465022
const updateRef = <T,>(ref: Ref<T> | undefined, node: T | undefined) => {
  if (!node) return;
  if (typeof ref === 'function') ref(node);
  else if (ref) ref.current = node;
};

export interface UseLinkProps extends ActiveLinkOptions {
  children?: ((state: { isActive: boolean; isTransitioning: boolean }) => React.ReactNode) | React.ReactNode;
  onAuxClick?: () => void;
  ref?: Ref<unknown> | undefined;
}

export const useLink = ({ children, onAuxClick, ref: refProp, ...props }: UseLinkProps) => {
  const _ = useLinkProps(props, refProp as Ref<Element>) as ComponentProps<'a'> & Record<string, unknown>;

  const isActive = _['data-status'] === 'active';
  const isTransitioning = _['data-transitioning'] === 'transitioning';

  const onAction = fauxEvent(_.onClick, { button: 0 });

  const ref = (node: unknown) => {
    updateRef(_.ref, node as HTMLAnchorElement);

    let element: HTMLElement | undefined;
    if (node && typeof node === 'object' && 'addEventListener' in node) element = node as HTMLElement;

    const onAuxClickHandler = (event: MouseEvent) => {
      event.preventDefault();
      if (onAuxClick) onAuxClick();
      else onAction();
    };

    element?.addEventListener('auxclick', onAuxClickHandler);
    return () => element?.removeEventListener('auxclick', onAuxClickHandler);
  };

  return {
    ref,

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

interface TabItemProps extends TabsRouteContext, ToOptions {
  id: string;
  tab: Tab;
}

const TabItem = ({ baseRoute, id, runtime, tab, tabsRx }: TabItemProps) => {
  const removeTab = useRemoveTab();

  const { isActive, ...linkProps } = useLink({
    ...tab.route,
    onAuxClick: () => void removeTab({ baseRoute, id: tab.id, runtime, tabsRx }),
  });

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
          void removeTab({ baseRoute, id: tab.id, runtime, tabsRx });
        }}
        variant='ghost'
      >
        <FiX className={tw`size-4 text-slate-500`} />
      </Button>
    </ListBoxItem>
  );
};

interface RouteTabListProps extends TabsRouteContext {}

export const RouteTabList = (props: RouteTabListProps) => {
  const tabs = useRxValue(props.tabsRx);

  useTabShortcuts(props);

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
      {(_) => <TabItem id={_.id} tab={_} {...props} />}
    </ListBox>
  );
};
