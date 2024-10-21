import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { Array, pipe, Predicate, Record } from 'effect';
import { Collection, UNSTABLE_Tree, UNSTABLE_TreeItem, UNSTABLE_TreeItemContent } from 'react-aria-components';

import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
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

  return (
    <UNSTABLE_Tree items={Array.fromRecord({ body, headers })} className={tw`flex flex-col gap-1`}>
      {function renderItem(
        [key, value]: readonly [string, unknown],
        parentKey?: string,
        parentKind?: 'object' | 'array',
      ) {
        const data = (() => {
          if (Predicate.isRecord(value)) {
            return { items: Array.fromRecord(value), kind: 'object' as const };
          } else if (Predicate.isIterable(value)) {
            return {
              items: Array.fromIterable(value).map((value, index) => [index.toString(), value] as const),
              kind: 'array' as const,
            };
          }
          return undefined;
        })();

        let keyDisplay: string | undefined = undefined;
        if (parentKind === 'array') keyDisplay = undefined;
        else keyDisplay = JSON.stringify(key);

        let tag: string | undefined = undefined;
        if (data?.kind !== undefined) tag = data.kind;
        else if (parentKind === 'array') tag = 'entry';
        if (parentKind === 'array') tag = `${tag} ${key}`;

        let quantity: string | undefined = undefined;
        if (data?.kind === 'object') quantity = `${data.items.length} keys`;
        else if (data?.kind === 'array') quantity = `${data.items.length} entries`;

        let valueDisplay: string | undefined = undefined;
        if (data?.kind === undefined) valueDisplay = JSON.stringify(value);

        return (
          <UNSTABLE_TreeItem id={`${parentKey} ${key}`} textValue={key}>
            <UNSTABLE_TreeItemContent>
              {({ level }) => (
                <div
                  className={tw`flex items-center gap-2`}
                  style={{ marginInlineStart: (level - 1).toString() + 'rem' }}
                >
                  {(data?.items.length ?? 0) > 0 && (
                    <Button slot='chevron' variant='placeholder ghost' kind='placeholder'>
                      +
                    </Button>
                  )}

                  {keyDisplay && <span className={tw`font-mono text-red-700`}>{keyDisplay}</span>}
                  {tag && <span className={tw`bg-gray-300 p-1`}>{tag}</span>}
                  {quantity && <span className={tw`text-gray-700`}>{quantity}</span>}

                  {valueDisplay && (
                    <>
                      : <span className={tw`font-mono text-blue-700`}>{valueDisplay}</span>
                    </>
                  )}
                </div>
              )}
            </UNSTABLE_TreeItemContent>
            {<Collection items={data?.items ?? []}>{(_) => renderItem(_, key, data?.kind)}</Collection>}
          </UNSTABLE_TreeItem>
        );
      }}
    </UNSTABLE_Tree>
  );
}
