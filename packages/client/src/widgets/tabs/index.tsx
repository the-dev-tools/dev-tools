import { createCollection, localOnlyCollectionOptions, useLiveQuery } from '@tanstack/react-db';
import { AnyRouter, linkOptions, RouteMatch, ToOptions, useRouter } from '@tanstack/react-router';
import { Array, Match, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useEffect } from 'react';
import * as RAC from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { Primitive } from '@the-dev-tools/ui';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorVertical } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { eqStruct, getNextOrder, handleCollectionReorderBasic, pick, queryCollection } from '~/shared/lib';
import { routes } from '~/shared/routes';

export interface Tab {
  id: string;
  node: ReactNode;
  order: number;
  route: ToOptions;
  workspaceId: Uint8Array;
}

export const tabCollection = createCollection(
  localOnlyCollectionOptions({
    getKey: (tab: Tab) => tab.id,
  }),
);

const baseRoute = (workspaceId: Uint8Array) =>
  linkOptions({
    params: { workspaceIdCan: Ulid.construct(workspaceId).toCanonical() },
    to: '/workspace/$workspaceIdCan',
  });

interface OpenTabProps {
  id: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  match: RouteMatch<any, any, { workspaceIdCan: string }, any, any, any, any>;
  node: ReactNode;
}

export const openTab = async ({ id, match, node }: OpenTabProps) => {
  const workspaceId = Ulid.fromCanonical(match.params.workspaceIdCan).bytes;

  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
  const route: ToOptions = { params: match.params, search: match.search, to: match.fullPath };

  if (tabCollection.has(id)) {
    tabCollection.update(id, (_) => (_.route = route as never));
  } else {
    tabCollection.insert({
      id,
      node,
      order: await getNextOrder(tabCollection),
      route,
      workspaceId,
    });
  }
};

export const useCloseTab = () => {
  const router: AnyRouter = useRouter();

  return async (id: string) => {
    const tab = tabCollection.get(id);
    if (!tab) return;

    const { workspaceId } = tab;

    let tabs = await queryCollection((_) =>
      _.from({ item: tabCollection })
        .where(eqStruct({ workspaceId }))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'id', 'order')),
    );

    const index = Array.findFirstIndex(tabs, (_) => _.id === id);
    if (Option.isNone(index)) return;

    tabCollection.delete(id);
    tabs = Array.remove(tabs, index.value);

    const match: unknown = router.matchRoute(tab.route);
    if (match === false) return;

    const nextTab = pipe(
      Array.get(tabs, index.value),
      Option.orElse(() => Array.last(tabs)),
      Option.flatMapNullable((_) => tabCollection.get(_.id)),
    );

    if (Option.isNone(nextTab)) {
      void router.navigate(baseRoute(workspaceId));
    } else {
      void router.navigate(nextTab.value.route);
    }
  };
};

interface TabItemProps {
  id: string;
}

const TabItem = ({ id }: TabItemProps) => {
  const closeTab = useCloseTab();

  const tab = useLiveQuery(
    (_) =>
      _.from({ item: tabCollection })
        .where(eqStruct({ id }))
        .select((_) => pick(_.item, 'route', 'node'))
        .findOne(),
    [id],
  ).data;

  if (!tab) return null;

  return (
    <Primitive.ListBoxItemRouteLink
      {...(tab.route as ToOptions)}
      aria-label='Tab'
      className={tw`
        relative -ml-px flex h-11 max-w-60 cursor-pointer items-center justify-between gap-3 border p-2.5 text-xs
        leading-4 font-medium tracking-tight text-fg

        not-route-active:border-b not-route-active:border-transparent not-route-active:border-b-border
        not-route-active:opacity-60

        before:absolute before:-left-px before:h-6 before:w-px before:bg-border

        route-active:rounded-t-md route-active:border route-active:border-border route-active:border-b-transparent
        route-active:bg-surface
      `}
      id={id}
      onAuxClick={(event) => {
        event.preventDefault();
        void closeTab(id);
      }}
    >
      <div className={tw`flex min-w-0 flex-1 items-center gap-1.5`}>{tab.node}</div>

      <Button
        className={tw`p-0.5`}
        onPress={(event) => {
          event.continuePropagation();
          void closeTab(id);
        }}
        variant='ghost'
      >
        <FiX className={tw`size-4 text-fg-muted`} />
      </Button>
    </Primitive.ListBoxItemRouteLink>
  );
};

export const RouteTabList = () => {
  const router: AnyRouter = useRouter();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const { data: tabs } = useLiveQuery(
    (_) =>
      _.from({ item: tabCollection })
        .where(eqStruct({ workspaceId }))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'id', 'order')),
    [workspaceId],
  );

  const { dragAndDropHooks } = RAC.useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorderBasic(tabCollection, (item, order) =>
      tabCollection.update(item.id, (_) => {
        _.order = order;
      }),
    ),
    renderDropIndicator: () => <DropIndicatorVertical />,
  });

  useEffect(() => {
    const onKeyDown = async (event: KeyboardEvent) => {
      const { code, ctrlKey, metaKey, shiftKey } = event;
      let shortcut: 'close' | 'next' | 'prev' | undefined;
      if ((ctrlKey || metaKey) && code === 'Tab') shortcut = 'next';
      if ((ctrlKey || metaKey) && shiftKey && code === 'Tab') shortcut = 'prev';
      if ((ctrlKey || metaKey) && code === 'KeyW') shortcut = 'close';
      if (!shortcut) return;

      event.preventDefault();

      let tabs = await queryCollection((_) =>
        _.from({ item: tabCollection })
          .where(eqStruct({ workspaceId }))
          .orderBy((_) => _.item.order)
          .select((_) => pick(_.item, 'id', 'order', 'route')),
      );

      const foundTab = Array.findFirstWithIndex(tabs, (_) => router.matchRoute(_.route as ToOptions) !== false);
      if (Option.isNone(foundTab)) return;
      const [{ id }, index] = foundTab.value;

      if (shortcut === 'close') {
        tabCollection.delete(id);
        tabs = Array.remove(tabs, index);
      }

      const tab = pipe(
        Match.value(shortcut),
        Match.when('close', () =>
          pipe(
            Array.get(tabs, index),
            Option.orElse(() => Array.last(tabs)),
          ),
        ),
        Match.when('next', () =>
          pipe(
            Array.get(tabs, index + 1),
            Option.orElse(() => Array.head(tabs)),
          ),
        ),
        Match.when('prev', () =>
          pipe(
            Array.get(tabs, index - 1),
            Option.orElse(() => Array.last(tabs)),
          ),
        ),
        Match.exhaustive,
      );

      if (Option.isNone(tab)) {
        void router.navigate(baseRoute(workspaceId));
      } else {
        void router.navigate(tab.value.route as ToOptions);
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => void window.removeEventListener('keydown', onKeyDown);
  }, [router, workspaceId]);

  return (
    <RAC.ListBox
      aria-label='Tabs'
      className={tw`
        relative flex h-11 w-full overflow-x-auto overflow-y-hidden

        before:absolute before:bottom-0 before:w-full before:border-b before:border-border
      `}
      dragAndDropHooks={dragAndDropHooks}
      items={tabs}
      orientation='horizontal'
      selectionMode='none'
      style={{ scrollbarWidth: 'thin' }}
    >
      {(_) => <TabItem id={_.id} />}
    </RAC.ListBox>
  );
};
