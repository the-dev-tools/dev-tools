import { Atom, Registry, useAtom } from '@effect-atom/atom-react';
import {
  ActiveLinkOptions,
  AnyRouteMatch,
  LinkComponent as LinkComponentUpstream,
  ToOptions,
  useLinkProps,
  useRouter,
} from '@tanstack/react-router';
import { Array, Effect, Match, Option, pipe, Runtime } from 'effect';
import React, {
  ComponentProps,
  MouseEventHandler,
  PropsWithChildren,
  ReactNode,
  Ref,
  Suspense,
  SyntheticEvent,
  useEffect,
} from 'react';
import { ListBox, ListBoxItem, RouterProvider, useDragAndDrop } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import { Button } from './button';
import { DropIndicatorVertical } from './reorder';
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
      currentTarget: {},
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

export type TabsAtom = ReturnType<typeof makeTabsAtom>;

export const makeTabsAtom = () => pipe(Array.empty<Tab>(), (_) => Atom.make(_), Atom.keepAlive);

export interface TabsRouteContext {
  baseRoute: ToOptions;
  runtime: Runtime.Runtime<Registry.AtomRegistry>;
  tabsAtom: TabsAtom;
}

interface AddTabProps {
  id: string;
  match: Omit<AnyRouteMatch, 'context'> & { context: TabsRouteContext };
  node: ReactNode;
}

export const addTab = ({ id, match, node }: AddTabProps) => {
  const { runtime, tabsAtom } = match.context;
  const tab: Tab = {
    id,
    node,
    route: {
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

  pipe(Atom.update(tabsAtom, updateTabs), Runtime.runSync(runtime));
};

interface RemoveTabProps extends TabsRouteContext {
  id: string;
}

export const useRemoveTab = () => {
  const router = useRouter();

  return async ({ baseRoute, id, runtime, tabsAtom }: RemoveTabProps) =>
    Effect.gen(function* () {
      let tabs = yield* Atom.get(tabsAtom);

      const index = Array.findFirstIndex(tabs, (_) => _.id === id);
      if (Option.isNone(index)) return;
      const tab = Array.unsafeGet(tabs, index.value);

      tabs = Array.remove(tabs, index.value);
      yield* Atom.set(tabsAtom, tabs);

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

const useTabShortcuts = ({ baseRoute, runtime, tabsAtom }: UseTabShortcutsProps) => {
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

        let tabs = yield* Atom.get(tabsAtom);
        const index = Array.findFirstIndex(tabs, (_) => router.matchRoute(_.route) !== false);

        if (Option.isNone(index)) return;

        if (shortcut === 'close') {
          tabs = Array.remove(tabs, index.value);
          yield* Atom.set(tabsAtom, tabs);
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
  }, [baseRoute, router, runtime, tabsAtom]);
};

// https://github.com/facebook/react/issues/29832#issuecomment-2490465022
const updateRef = <T,>(ref: Ref<T> | undefined, node: T | undefined) => {
  if (!node) return;
  if (typeof ref === 'function') ref(node);
  else if (ref) ref.current = node;
};

export interface UseLinkProps extends ActiveLinkOptions {
  children?: ((state: { isActive: boolean; isTransitioning: boolean }) => React.ReactNode) | React.ReactNode;
  onAuxClick?: MouseEventHandler<HTMLDivElement>;
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

    const onClickHandler = (event: MouseEvent) => {
      // Prevent opening links in external browser
      if (event.ctrlKey) event.preventDefault();
    };

    const onAuxClickHandler = (event: MouseEvent) => {
      event.preventDefault();
      if (onAuxClick) fauxEvent(onAuxClick)(event);
      else onAction();
    };

    element?.addEventListener('click', onClickHandler);
    element?.addEventListener('auxclick', onAuxClickHandler);

    return () => {
      element?.removeEventListener('click', onClickHandler);
      element?.removeEventListener('auxclick', onAuxClickHandler);
    };
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

const TabItem = ({ baseRoute, id, runtime, tab, tabsAtom }: TabItemProps) => {
  const removeTab = useRemoveTab();

  const { isActive, ...linkProps } = useLink({
    ...tab.route,
    onAuxClick: () => void removeTab({ baseRoute, id: tab.id, runtime, tabsAtom }),
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
          void removeTab({ baseRoute, id: tab.id, runtime, tabsAtom });
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
  const [tabs, setTabs] = useAtom(props.tabsAtom);

  useTabShortcuts(props);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) => {
      setTabs((tabs) =>
        Option.gen(function* () {
          const offset = yield* pipe(
            Match.value(dropPosition),
            Match.when('after', () => 1),
            Match.when('before', () => 0),
            Match.option,
          );

          const { rest = [], selection = [] } = Array.groupBy(tabs, (_) => (keys.has(_.id) ? 'selection' : 'rest'));

          const index = yield* Array.findFirstIndex(rest, (_) => _.id === key);

          const [before, after] = Array.splitAt(rest, index + offset);

          return [...before, ...selection, ...after];
        }).pipe(Option.getOrElse(() => tabs)),
      );
    },
    renderDropIndicator: () => <DropIndicatorVertical />,
  });

  return (
    <ListBox
      aria-label='Tabs'
      className={tw`
        relative flex h-11 w-full overflow-auto

        before:absolute before:bottom-0 before:w-full before:border-b before:border-gray-200
      `}
      dragAndDropHooks={dragAndDropHooks}
      items={tabs}
      orientation='horizontal'
      selectionMode='none'
    >
      {(_) => <TabItem id={_.id} tab={_} {...props} />}
    </ListBox>
  );
};
