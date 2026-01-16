import { Atom, Registry, useAtom } from '@effect-atom/atom-react';
import { AnyRouteMatch, ToOptions, useRouter } from '@tanstack/react-router';
import { Array, Effect, Match, Option, pipe, Runtime } from 'effect';
import { ReactNode, Suspense, useEffect } from 'react';
import * as RAC from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { Button } from './button';
import * as Primitive from './primitives';
import { DropIndicatorVertical } from './reorder';
import { tw } from './tailwind-literal';

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

interface TabItemProps extends TabsRouteContext, ToOptions {
  id: string;
  tab: Tab;
}

const TabItem = ({ baseRoute, id, runtime, tab, tabsAtom }: TabItemProps) => {
  const removeTab = useRemoveTab();

  return (
    <Primitive.ListBoxItemRouteLink
      {...tab.route}
      aria-label='Tab'
      className={tw`
        relative -ml-px flex h-11 max-w-60 cursor-pointer items-center justify-between gap-3 border p-2.5 text-xs
        leading-4 font-medium tracking-tight text-slate-800

        not-route-active:border-b not-route-active:border-transparent not-route-active:border-b-gray-200
        not-route-active:opacity-60

        before:absolute before:-left-px before:h-6 before:w-px before:bg-gray-200

        route-active:rounded-t-md route-active:border route-active:border-gray-200 route-active:border-b-transparent
        route-active:bg-white
      `}
      id={id}
      onAuxClick={(event) => {
        event.preventDefault();
        void removeTab({ baseRoute, id: tab.id, runtime, tabsAtom });
      }}
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
    </Primitive.ListBoxItemRouteLink>
  );
};

interface RouteTabListProps extends TabsRouteContext {}

export const RouteTabList = (props: RouteTabListProps) => {
  const [tabs, setTabs] = useAtom(props.tabsAtom);

  useTabShortcuts(props);

  const { dragAndDropHooks } = RAC.useDragAndDrop({
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
    <RAC.ListBox
      aria-label='Tabs'
      className={tw`
        relative flex h-11 w-full overflow-x-auto overflow-y-hidden

        before:absolute before:bottom-0 before:w-full before:border-b before:border-gray-200
      `}
      dragAndDropHooks={dragAndDropHooks}
      items={tabs}
      orientation='horizontal'
      selectionMode='none'
      style={{ scrollbarWidth: 'thin' }}
    >
      {(_) => <TabItem id={_.id} tab={_} {...props} />}
    </RAC.ListBox>
  );
};
