import { create, fromJson, toJson } from '@bufbuild/protobuf';
import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { Array, Match, pipe, Predicate, Record, Tuple } from 'effect';
import { useMemo } from 'react';
import {
  Collection as AriaCollection,
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItem as AriaTreeItem,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { LuChevronRight } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import {
  PathKey,
  PathKeyJson,
  PathKeySchema,
  PathKind,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan/assertions',
)({
  component: Tab,
});

const endpointRoute = getRouteApi(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
);

export function Tab() {
  const { exampleId } = endpointRoute.useLoaderData();

  const exampleQuery = useConnectQuery(exampleGet, { exampleId });

  const responseId = exampleQuery.data?.lastResponseId;
  const hasResponse = responseId !== undefined;
  const input = hasResponse ? { responseId } : {};

  const responseQuery = useConnectQuery(responseGet, input, { enabled: hasResponse });
  const headersQuery = useConnectQuery(responseHeaderList, input, { enabled: hasResponse });

  if (!responseQuery.isSuccess || !headersQuery.isSuccess) return null;

  let body;
  try {
    body = new TextDecoder().decode(responseQuery.data.body);
    body = JSON.parse(body) as unknown;
    if (typeof body !== 'object') body = null;
  } catch {
    body = null;
  }

  const headers = pipe(
    headersQuery.data.items,
    Array.map((_) => [_.key, _.value] as const),
    Record.fromEntries,
  );

  const items = pipe(
    Array.fromRecord({ body, headers }),
    Array.map(([key, data]) => {
      const path = Array.make(create(PathKeySchema, { key }));
      const ids = path.map((_) => toJson(PathKeySchema, _));
      return { id: JSON.stringify(ids), data, path };
    }),
  );

  return (
    <AriaTree
      items={items}
      className={tw`flex flex-col gap-1`}
      onAction={(id) => {
        if (typeof id !== 'string') return;
        const path = pipe(
          JSON.parse(id) as PathKeyJson[],
          Array.map((_) => fromJson(PathKeySchema, _)),
        );
        console.log(...path);
      }}
    >
      {({ id, data, path }) => <PathTreeItem id={id} data={data} path={path} />}
    </AriaTree>
  );
}

interface PathTreeItemProps {
  id: string;
  data: unknown;
  path: Array.NonEmptyArray<PathKey>;
}

const PathTreeItem = ({ id, data, path }: PathTreeItemProps) => {
  const value = useMemo(
    () =>
      pipe(
        Match.value(data),
        Match.when(Predicate.isRecord, (_) => ({
          kind: 'object' as const,
          items: pipe(Array.fromRecord(_), Array.map(Tuple.mapFirst((_) => create(PathKeySchema, { key: _ })))),
        })),
        Match.when(Predicate.isIterable, (_) => ({
          kind: 'array' as const,
          items: pipe(
            Array.fromIterable(_),
            Array.map((data, index) => [create(PathKeySchema, { kind: PathKind.INDEX, index }), data] as const),
            // Array.prepend([create(PathKeySchema, { kind: PathKind.INDEX_ANY }), null] as const), // TODO: construct 'any' object
          ),
        })),
        Match.orElse((_) => ({ kind: 'unknown' as const, value: _ })),
      ),
    [data],
  );

  const items = useMemo(
    () =>
      pipe(
        value.kind !== 'unknown' ? value.items : [],
        Array.map(([key, data]) => {
          const itemPath = Array.append(path, key);
          const ids = itemPath.map((_) => toJson(PathKeySchema, _));
          return { id: JSON.stringify(ids), data, path: itemPath };
        }),
      ),
    [path, value],
  );

  const key = Array.lastNonEmpty(path);

  const keyDisplay = pipe(
    Match.value(key),
    Match.when({ kind: PathKind.UNSPECIFIED }, (_) => JSON.stringify(_.key)),
    Match.orElse(() => undefined),
  );

  let tag: string | undefined = undefined;
  if (value.kind !== 'unknown') tag = value.kind;
  else if (key.kind !== PathKind.UNSPECIFIED) tag = 'entry';
  if (key.kind !== PathKind.UNSPECIFIED) tag = `${tag} ${key.index}`;

  const quantity = pipe(
    Match.value(value),
    Match.when({ kind: 'object' }, (_) => `${_.items.length} keys`),
    Match.when({ kind: 'array' }, (_) => `${_.items.length} entries`),
    Match.orElse(() => undefined),
  );

  const valueDisplay = pipe(
    Match.value(value),
    Match.when({ kind: 'unknown' }, (_) => JSON.stringify(_.value)),
    Match.orElse(() => undefined),
  );

  return (
    <AriaTreeItem id={id} textValue={valueDisplay ?? tag ?? ''}>
      <AriaTreeItemContent>
        {({ level, isExpanded }) => (
          <div className={tw`flex items-center gap-2`} style={{ marginInlineStart: (level - 1).toString() + 'rem' }}>
            {items.length > 0 && (
              <Button kind='placeholder' variant='placeholder ghost' slot='chevron'>
                <LuChevronRight
                  className={twJoin(tw`transition-transform`, !isExpanded ? tw`rotate-0` : tw`rotate-90`)}
                />
              </Button>
            )}

            {keyDisplay && <span className={tw`font-mono text-red-700`}>{keyDisplay}</span>}
            {tag && <span className={tw`bg-gray-300 p-1`}>{tag}</span>}
            {quantity && <span className={tw`text-gray-700`}>{quantity}</span>}

            {valueDisplay && (
              <>
                : <span className={tw`flex-1 break-all font-mono text-blue-700`}>{valueDisplay}</span>
              </>
            )}
          </div>
        )}
      </AriaTreeItemContent>
      <AriaCollection items={items}>
        {({ id, data, path }) => <PathTreeItem id={id} data={data} path={path} />}
      </AriaCollection>
    </AriaTreeItem>
  );
};
