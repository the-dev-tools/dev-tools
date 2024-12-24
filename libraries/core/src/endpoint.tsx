import { create, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useMutation as useConnectMutation,
  useSuspenseQuery as useConnectSuspenseQuery,
} from '@connectrpc/connect-query';
import { makeUrl } from '@effect/platform/UrlParams';
import { useQuery, useQueryClient, useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, redirect, useRouteContext } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, Either, HashMap, Match, MutableHashMap, Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { format as prettierFormat } from 'prettier/standalone';
import { Fragment, Suspense, useMemo, useState } from 'react';
import { MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiChevronDown, FiClock, FiLink, FiMoreHorizontal, FiSave, FiSidebar, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { queryCreateSpec } from '@the-dev-tools/api/spec/collection/item/request';
import { PathKeySchema } from '@the-dev-tools/spec/assert/v1/assert_pb';
import {
  endpointGet,
  endpointUpdate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import {
  exampleGet,
  exampleRun,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import {
  QueryCreateRequest,
  QueryCreateRequestSchema,
  QueryListItemSchema,
  QueryListResponseSchema,
  QueryUpdateRequest,
  QueryUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { ResponseHeaderListItem } from '@the-dev-tools/spec/collection/item/response/v1/response_pb';
import {
  responseAssertList,
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select, SelectRHF } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { AssertionView } from './assertions';
import { BodyView } from './body';
import { HeaderTable } from './headers';
import { QueryTable } from './query';

export class EndpointRouteSearch extends Schema.Class<EndpointRouteSearch>('EndpointRouteSearch')({
  requestTab: pipe(
    Schema.Literal('params', 'headers', 'body', 'assertions'),
    Schema.optionalWith({ default: () => 'params' }),
  ),
  responseTab: pipe(Schema.Literal('body', 'headers', 'assertions'), Schema.optionalWith({ default: () => 'body' })),
  responseIdCan: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
)({
  component: Page,
  pendingComponent: () => 'Loading example...',
  shouldReload: false,
  validateSearch: (_) => Schema.decodeSync(EndpointRouteSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'responseIdCan'),
  loader: async ({
    params: { workspaceIdCan, endpointIdCan, exampleIdCan },
    deps: { responseIdCan },
    context: { transport, queryClient },
  }) => {
    const endpointId = Ulid.fromCanonical(endpointIdCan).bytes;
    const exampleId = Ulid.fromCanonical(exampleIdCan).bytes;
    const responseId = pipe(
      Option.fromNullable(responseIdCan),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(exampleGet, { exampleId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(endpointGet, { endpointId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(queryList, { exampleId }, { transport })),
        ...pipe(
          Option.map(responseId, (_) =>
            queryClient.ensureQueryData(createQueryOptions(responseGet, { responseId: _ }, { transport })),
          ),
          Option.toArray,
        ),
      ]);
    } catch {
      redirect({
        to: '/workspace/$workspaceIdCan',
        params: { workspaceIdCan },
        throw: true,
      });
    }

    return { endpointId, exampleId, responseId };
  },
});

function Page() {
  const { endpointId, exampleId } = Route.useLoaderData();
  const { requestTab, responseTab } = Route.useSearch();

  const { data: example } = useConnectSuspenseQuery(exampleGet, { exampleId });

  return (
    <PanelGroup direction='vertical'>
      <Panel id='request' order={1} className='flex h-full flex-col'>
        <EndpointForm endpointId={endpointId} exampleId={exampleId} />

        <EndpointRequestView endpointId={endpointId} exampleId={exampleId} requestTab={requestTab} />
      </Panel>
      {example.lastResponseId.byteLength > 0 && (
        <>
          <PanelResizeHandle direction='vertical' />
          <Panel id='response' order={2} defaultSize={40}>
            <Suspense fallback='Loading response...'>
              <ResponsePanel responseId={example.lastResponseId} responseTab={responseTab} fullWidth />
            </Suspense>
          </Panel>
        </>
      )}
    </PanelGroup>
  );
}

interface EndpointRequestViewProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
  requestTab: EndpointRouteSearch['requestTab'];
  className?: string;
}

export const EndpointRequestView = ({ endpointId, exampleId, requestTab, className }: EndpointRequestViewProps) => (
  <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)} selectedKey={requestTab}>
    <TabList className={tw`flex gap-3 border-b border-slate-200`}>
      <Tab
        id='params'
        href={{
          to: '.',
          search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, requestTab: 'params' }),
        }}
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Params
      </Tab>

      <Tab
        id='headers'
        href={{
          to: '.',
          search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, requestTab: 'headers' }),
        }}
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Headers
      </Tab>

      <Tab
        id='body'
        href={{
          to: '.',
          search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, requestTab: 'body' }),
        }}
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Body
      </Tab>

      <Tab
        id='assertions'
        href={{
          to: '.',
          search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, requestTab: 'assertions' }),
        }}
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Assertion
      </Tab>
    </TabList>

    <Suspense fallback='Loading tab...'>
      <TabPanel id='params'>
        <QueryTable exampleId={exampleId} />
      </TabPanel>

      <TabPanel id='headers'>
        <HeaderTable exampleId={exampleId} />
      </TabPanel>

      <TabPanel id='body'>
        <BodyView endpointId={endpointId} exampleId={exampleId} />
      </TabPanel>

      <TabPanel id='assertions'>
        <AssertionView exampleId={exampleId} />
      </TabPanel>
    </Suspense>
  </Tabs>
);

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

interface UseEndpointUrlProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const useEndpointUrl = ({ endpointId, exampleId }: UseEndpointUrlProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const [
    { data: endpoint },
    {
      data: { items: queries },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(queryList, { exampleId }, { transport }),
    ],
  });

  return useMemo(() => {
    return pipe(
      Option.fromNullable(endpoint.url || null),
      Option.flatMap((url) =>
        pipe(
          Array.filterMap(queries, (_) => {
            if (!_.enabled) return Option.none();
            else return Option.some([_.key, _.value] as const);
          }),
          (_) => makeUrl(url, _, Option.none()),
          Either.getRight,
        ),
      ),
      Option.map((_) => _.toString()),
      Option.getOrElse(() => endpoint.url),
    );
  }, [endpoint.url, queries]);
};

class EndpointFormData extends Schema.Class<EndpointFormData>('EndpointFormData')({
  method: Schema.String,
  url: Schema.String,
}) {}

interface EndpointFormProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const EndpointForm = ({ endpointId, exampleId }: EndpointFormProps) => {
  const { data: endpoint } = useConnectSuspenseQuery(endpointGet, { endpointId });
  const {
    data: { items: queries },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const queryClient = useQueryClient();

  const endpointUpdateMutation = useConnectMutation(endpointUpdate);
  const exampleRunMutation = useConnectMutation(exampleRun);

  const queryUpdateMutation = useConnectMutation(queryUpdate);
  const queryCreateMutation = useSpecMutation(queryCreateSpec);

  const url = useEndpointUrl({ endpointId, exampleId });

  const values = useMemo(() => {
    return new EndpointFormData({
      url,
      method: Array.contains(methods, endpoint.method) ? endpoint.method : 'N/A',
    });
  }, [endpoint.method, url]);

  const form = useForm({
    // TODO: use Effect resolver once it's updated
    // https://github.com/react-hook-form/resolvers/pull/720
    // resolver: effectTsResolver(EndpointFormData),
    values,
  });

  const onSubmit = form.handleSubmit(async ({ method, url: urlString }) => {
    const { origin = '', pathname = '', searchParams = new URLSearchParams() } = !urlString ? {} : new URL(urlString);

    endpointUpdateMutation.mutate({ endpointId, method, url: origin + pathname });

    const queryMap = pipe(
      searchParams.entries(),
      Array.fromIterable,
      Array.map(([key, value]): [string, QueryCreateRequest | QueryUpdateRequest] => [
        key + value,
        create(QueryCreateRequestSchema, { key, value }),
      ]),
      MutableHashMap.fromIterable,
    );

    queries.forEach(({ queryId, key, value, enabled }) => {
      MutableHashMap.modifyAt(
        queryMap,
        key + value,
        Option.match({
          onSome: () => {
            if (enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { queryId, enabled: true }));
          },
          onNone: () => {
            if (!enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { queryId, enabled: false }));
          },
        }),
      );
    });

    const queryIdIndexMap = pipe(
      queries,
      Array.map(({ queryId }, index) => [Ulid.construct(queryId).toRaw(), index] as const),
      HashMap.fromIterable,
    );

    const newQueryList = Array.copy(queries);
    await pipe(
      Array.fromIterable(queryMap),
      Array.map(async ([_, query]) => {
        if (query.$typeName === 'collection.item.request.v1.QueryUpdateRequest') {
          await queryUpdateMutation.mutateAsync(query);
          const index = HashMap.unsafeGet(queryIdIndexMap, Ulid.construct(query.queryId).toRaw());
          const oldQuery = newQueryList[index];
          if (!oldQuery) return;
          newQueryList[index] = create(QueryListItemSchema, {
            ...oldQuery,
            ...Struct.omit(query, '$typeName'),
          });
        } else {
          const { queryId } = await queryCreateMutation.mutateAsync(query);
          newQueryList.push(
            create(QueryListItemSchema, {
              queryId,
              ...Struct.omit(query, '$typeName'),
            }),
          );
        }
      }),
      (_) => Promise.allSettled(_),
    );

    queryClient.setQueryData(
      createConnectQueryKey({
        schema: queryList,
        cardinality: 'finite',
        input: { exampleId },
      }),
      createProtobufSafeUpdater(queryList, () => create(QueryListResponseSchema, { items: newQueryList })),
    );
  });

  return (
    <form onSubmit={onSubmit}>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        {/* TODO: implement breadcrumbs */}
        <div className={tw`flex flex-1 select-none gap-1 text-md font-medium leading-5 tracking-tight text-slate-400`}>
          {['Collection', 'Folder', 'Endpoint'].map((_) => (
            <Fragment key={_}>
              <span className={tw`cursor-pointer`}>{_}</span>
              <span>/</span>
            </Fragment>
          ))}

          <h2 className={tw`cursor-pointer text-slate-800`}>Example</h2>
        </div>

        {/* TODO: implement response history */}
        <Button variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiClock className={tw`size-4 text-slate-500`} /> Response History
        </Button>

        {/* TODO: implement copy link */}
        <Button variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiLink className={tw`size-4 text-slate-500`} /> Copy Link
        </Button>

        <Separator orientation='vertical' className={tw`h-4`} />

        <Button type='submit' variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiSave className={tw`size-4 text-slate-500`} /> Save
        </Button>

        {/* TODO: implement overflow menu item functionality */}
        <MenuTrigger>
          <Button variant='ghost' className={tw`p-1`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu>
            <MenuItem>Add example</MenuItem>
            <Separator />
            <MenuItem>Rename</MenuItem>
            <MenuItem>View Documentation</MenuItem>
            <MenuItem variant='danger'>Delete</MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <div className='flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-sm'>
          <SelectRHF control={form.control} name='method' aria-label='Method' triggerClassName={tw`border-none p-0`}>
            {methods.map((_) => (
              <ListBoxItem key={_} id={_} textValue={_}>
                <MethodBadge method={_} size='lg' />
              </ListBoxItem>
            ))}
          </SelectRHF>

          <Separator orientation='vertical' className={tw`h-7`} />

          {/* TODO: update styles after component is refactored */}
          <TextFieldRHF
            control={form.control}
            onBlur={onSubmit}
            name='url'
            aria-label='URL'
            className={tw`flex-1`}
            inputClassName={tw`border-none bg-transparent font-medium leading-5 tracking-tight text-slate-800`}
          />
        </div>

        <Button
          variant='primary'
          className={tw`px-6`}
          onPress={async () => {
            await onSubmit();
            const { responseId } = await exampleRunMutation.mutateAsync({
              exampleId,
            });
            queryClient.setQueryData(
              createConnectQueryKey({
                schema: exampleGet,
                cardinality: 'finite',
                input: { exampleId },
              }),
              createProtobufSafeUpdater(exampleGet, (old) => {
                if (old === undefined) return undefined;
                return { ...old, lastResponseId: responseId };
              }),
            );
          }}
        >
          Send
        </Button>
      </div>
    </form>
  );
};

interface ResponsePanelProps {
  responseId: Uint8Array;
  responseTab: EndpointRouteSearch['responseTab'];
  showActions?: boolean;
  fullWidth?: boolean;
  className?: string;
}

export const ResponsePanel = ({
  responseId,
  responseTab,
  showActions = false,
  fullWidth = false,
  className,
}: ResponsePanelProps) => {
  const { data: response } = useConnectSuspenseQuery(responseGet, { responseId });

  return (
    <Tabs className={twMerge(tw`flex h-full flex-col pb-4`, className)} selectedKey={responseTab}>
      <div className={twMerge(tw`flex items-center gap-3 border-b border-slate-200 text-md`, fullWidth && tw`px-4`)}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            id='body'
            href={{
              to: '.',
              search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, responseTab: 'body' }),
            }}
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Body
          </Tab>

          <Tab
            id='headers'
            href={{
              to: '.',
              search: (_: Partial<EndpointRouteSearch>) => EndpointRouteSearch.make({ ..._, responseTab: 'headers' }),
            }}
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Headers
          </Tab>

          <Tab
            id='assertions'
            href={{
              to: '.',
              search: (_: Partial<EndpointRouteSearch>) =>
                EndpointRouteSearch.make({ ..._, responseTab: 'assertions' }),
            }}
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Test Results
          </Tab>
        </TabList>

        <div className={tw`flex-1`} />

        <div className={tw`flex items-center gap-1 text-xs font-medium leading-5 tracking-tight text-slate-800`}>
          <div className={tw`flex gap-1 p-2`}>
            <span>Status:</span>
            <span className={tw`text-green-600`}>{response.status}</span>
          </div>

          <Separator orientation='vertical' className={tw`h-4`} />

          <div className={tw`flex gap-1 p-2`}>
            <span>Time:</span>
            <span className={tw`text-green-600`}>
              {pipe(Number(response.duration), Duration.millis, Duration.format)}
            </span>
          </div>

          <Separator orientation='vertical' className={tw`h-4`} />

          {/* TODO: implement response size */}
          <div className={tw`flex gap-1 p-2`}>
            <span>Size:</span>
            <span>0.0 KB</span>
          </div>

          {showActions && (
            <>
              <Separator orientation='vertical' className={tw`h-4`} />

              {/* TODO: implement menu */}
              <Button variant='ghost' className={tw`px-2 text-xs`}>
                <FiSave className={tw`size-4 text-slate-500`} />
                <span>Save as</span>
                <FiChevronDown className={tw`size-4 text-slate-500`} />
              </Button>

              <Separator orientation='vertical' className={tw`h-4`} />

              {/* TODO: implement clear */}
              <Button variant='ghost' className={tw`px-2 text-xs`}>
                <FiX className={tw`size-4 text-slate-500`} />
                <span>Clear</span>
              </Button>

              {/* TODO: implement bottom card */}
              <Button variant='ghost' className={tw`p-1`}>
                <FiSidebar className={tw`size-4 text-slate-500`} />
              </Button>
            </>
          )}
        </div>
      </div>

      <div className={twJoin(tw`flex-1 overflow-auto pt-4`, fullWidth && tw`px-4`)}>
        <Suspense fallback='Loading tab...'>
          <TabPanel id='body' className={twJoin(tw`flex h-full flex-col gap-4`)}>
            <ResponseBodyView bodyBytes={response.body} />
          </TabPanel>

          <TabPanel id='headers'>
            <ResponseHeaderTable responseId={responseId} />
          </TabPanel>

          <TabPanel id='assertions'>
            <ResponseAssertTable responseId={responseId} />
          </TabPanel>
        </Suspense>
      </div>
    </Tabs>
  );
};

const languages = ['text', 'json', 'html', 'xml'] as const;

interface ResponseBodyViewProps {
  bodyBytes: Uint8Array;
}

const ResponseBodyView = ({ bodyBytes }: ResponseBodyViewProps) => {
  const body = new TextDecoder().decode(bodyBytes);

  return (
    <Tabs className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <TabList className='flex gap-1 self-start rounded-md border border-slate-100 bg-slate-100 p-0.5 text-xs leading-5 tracking-tight'>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='pretty'
        >
          Pretty
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='raw'
        >
          Raw
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='preview'
        >
          Preview
        </Tab>
      </TabList>

      <TabPanel id='pretty' className='contents'>
        <ResponseBodyPrettyView body={body} />
      </TabPanel>

      <TabPanel id='raw' className='col-span-full font-mono'>
        {body}
      </TabPanel>

      <TabPanel id='preview' className='col-span-full self-stretch'>
        <iframe title='Response preview' srcDoc={body} className='size-full' />
      </TabPanel>
    </Tabs>
  );
};

interface ResponseBodyPrettyViewProps {
  body: string;
}

const ResponseBodyPrettyView = ({ body }: ResponseBodyPrettyViewProps) => {
  const [language, setLanguage] = useState<(typeof languages)[number]>('text');

  const { data: prettierBody } = useQuery({
    initialData: '',
    queryKey: ['prettier', language, body],
    queryFn: async () => {
      if (language === 'text') return body;

      const plugins = await pipe(
        Match.value(language),
        Match.when('json', () => [import('prettier/plugins/estree'), import('prettier/plugins/babel')]),
        Match.when('html', () => [import('prettier/plugins/html')]),
        Match.when('xml', () => [import('@prettier/plugin-xml')]),
        Match.exhaustive,
        Array.map((_) => _.then((_) => _.default)),
        (_) => Promise.all(_),
      );

      const parser = pipe(
        Match.value(language),
        Match.when('json', () => 'json-stringify'),
        Match.orElse((_) => _),
      );

      return await prettierFormat(body, {
        parser,
        plugins,
        singleAttributePerLine: true,
        htmlWhitespaceSensitivity: 'ignore',
        xmlWhitespaceSensitivity: 'ignore',
      }).catch(() => body);
    },
  });

  const { data: extensions } = useQuery({
    initialData: [],
    queryKey: ['code-mirror', language],
    queryFn: async () => {
      if (language === 'text') return [];
      return await pipe(
        Match.value(language),
        Match.when('json', () => import('@codemirror/lang-json').then((_) => _.json())),
        Match.when('html', () => import('@codemirror/lang-html').then((_) => _.html())),
        Match.when('xml', () => import('@codemirror/lang-xml').then((_) => _.xml())),
        Match.exhaustive,
        (_) => _.then(Array.make),
      );
    },
  });

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
        triggerClassName={tw`px-4 py-1`}
        selectedKey={language}
        onSelectionChange={(_) => void setLanguage(_ as (typeof languages)[number])}
      >
        {languages.map((_) => (
          <ListBoxItem key={_} id={_}>
            {_}
          </ListBoxItem>
        ))}
      </Select>

      <CodeMirror
        value={prettierBody}
        readOnly
        height='100%'
        className='col-span-full self-stretch'
        extensions={extensions}
      />
    </>
  );
};

interface ResponseHeaderTableProps {
  responseId: Uint8Array;
}

const ResponseHeaderTable = ({ responseId }: ResponseHeaderTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(responseHeaderList, { responseId });

  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeaderListItem>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} cellClassName={tw`px-5 py-1.5`} />;
};

interface ResponseAssertTableProps {
  responseId: Uint8Array;
}

const ResponseAssertTable = ({ responseId }: ResponseAssertTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(responseAssertList, { responseId });

  return (
    <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
      {items.map(({ assert, result }) => {
        if (!assert) return null;
        const assertIdCan = Ulid.construct(assert.assertId).toCanonical();
        return (
          <Fragment key={assertIdCan}>
            <div
              className={twJoin(
                tw`rounded px-2 py-1 text-center font-light uppercase text-white`,
                result ? tw`bg-green-600` : tw`bg-red-600`,
              )}
            >
              {result ? 'Pass' : 'Fail'}
            </div>

            <span>{assert.path.map((_) => JSON.stringify(toJson(PathKeySchema, _))).join(' ')}</span>
          </Fragment>
        );
      })}
    </div>
  );
};
