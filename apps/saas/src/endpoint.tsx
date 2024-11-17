import { create, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { makeUrl } from '@effect/platform/UrlParams';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Link, Outlet, redirect } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, Either, HashMap, Match, MutableHashMap, Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { format as prettierFormat } from 'prettier/standalone';
import { Fragment, useMemo, useState } from 'react';
import { MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiChevronDown, FiClock, FiLink, FiMoreHorizontal, FiSave, FiSidebar, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { queryCreateSpec } from '@the-dev-tools/api/spec/collection/item/request';
import { EndpointGetResponse } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import {
  endpointGet,
  endpointUpdate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleGetResponse } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleGet,
  exampleRun,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import {
  PathKeySchema,
  QueryCreateRequest,
  QueryCreateRequestSchema,
  QueryListItemSchema,
  QueryListResponse,
  QueryListResponseSchema,
  QueryUpdateRequest,
  QueryUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import {
  Response,
  ResponseAssertListItem,
  ResponseGetResponse,
  ResponseHeaderListItem,
} from '@the-dev-tools/spec/collection/item/response/v1/response_pb';
import {
  responseAssertList,
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select, SelectRHF } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
)({
  component: Page,
  pendingComponent: () => 'Loading example...',
  loader: async ({ params: { workspaceIdCan, endpointIdCan, exampleIdCan }, context: { transport, queryClient } }) => {
    const endpointId = Ulid.fromCanonical(endpointIdCan).bytes;
    const exampleId = Ulid.fromCanonical(exampleIdCan).bytes;

    try {
      const [{ lastResponseId }] = await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(exampleGet, { exampleId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(endpointGet, { endpointId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(queryList, { exampleId }, { transport })),
      ]);

      if (lastResponseId.byteLength > 0) {
        await queryClient.ensureQueryData(
          createQueryOptions(responseGet, { responseId: lastResponseId }, { transport }),
        );
      }
    } catch {
      redirect({
        to: '/workspace/$workspaceIdCan',
        params: { workspaceIdCan },
        throw: true,
      });
    }

    return { endpointId, exampleId };
  },
});

function Page() {
  const { endpointId, exampleId } = Route.useLoaderData();

  const endpointGetQuery = useConnectQuery(endpointGet, { endpointId });
  const exampleGetQuery = useConnectQuery(exampleGet, { exampleId });
  const queryListQuery = useConnectQuery(queryList, { exampleId });

  if (!endpointGetQuery.isSuccess || !exampleGetQuery.isSuccess || !queryListQuery.isSuccess) return null;

  return (
    <EndpointForm endpoint={endpointGetQuery.data} example={exampleGetQuery.data} queries={queryListQuery.data.items} />
  );
}

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

class EndpointFormData extends Schema.Class<EndpointFormData>('EndpointFormData')({
  method: Schema.String,
  url: Schema.String,
}) {}

interface EndpointFormProps {
  endpoint: EndpointGetResponse;
  example: ExampleGetResponse;
  queries: QueryListResponse['items'];
}

const EndpointForm = ({ endpoint, example, queries }: EndpointFormProps) => {
  const { endpointId, exampleId } = Route.useLoaderData();

  const queryClient = useQueryClient();

  const endpointUpdateMutation = useConnectMutation(endpointUpdate);
  const exampleRunMutation = useConnectMutation(exampleRun);

  const queryUpdateMutation = useConnectMutation(queryUpdate);
  const queryCreateMutation = useSpecMutation(queryCreateSpec);

  const values = useMemo(() => {
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
      (_) =>
        new EndpointFormData({
          url: _,
          method: Array.contains(methods, endpoint.method) ? endpoint.method : 'N/A',
        }),
    );
  }, [endpoint.method, endpoint.url, queries]);

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
    <PanelGroup direction='vertical'>
      <Panel id='request' order={1} className='flex h-full flex-col'>
        <form onSubmit={onSubmit}>
          <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
            {/* TODO: implement breadcrumbs */}
            <div
              className={tw`flex flex-1 select-none gap-1 text-md font-medium leading-5 tracking-tight text-slate-400`}
            >
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
              <SelectRHF
                control={form.control}
                name='method'
                aria-label='Method'
                triggerClassName={tw`border-none p-0`}
              >
                {methods.map((_) => (
                  <ListBoxItem key={_} id={_}>
                    {/* TODO: use tag component */}
                    <div
                      className={tw`rounded-md border border-green-200 bg-green-100 px-3 py-1 text-xs font-semibold leading-4 tracking-tight text-teal-600`}
                    >
                      {_}
                    </div>
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

        <div className={tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`}>
          <div className={tw`flex gap-3 border-b border-slate-200`}>
            <Link
              className={tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`}
              activeProps={{ className: tw`border-b-violet-700 text-slate-800` }}
              activeOptions={{ exact: true }}
              from='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
              to='.'
            >
              Params
            </Link>
            <Link
              className={tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`}
              activeProps={{ className: tw`border-b-violet-700 text-slate-800` }}
              from='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
              to='headers'
            >
              Headers
            </Link>
            <Link
              className={tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`}
              activeProps={{ className: tw`border-b-violet-700 text-slate-800` }}
              from='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
              to='body'
            >
              Body
            </Link>
            <Link
              className={tw`-mb-px border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`}
              activeProps={{ className: tw`border-b-violet-700 text-slate-800` }}
              from='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
              to='assertions'
            >
              Assertion
            </Link>
          </div>

          <Outlet />
        </div>
      </Panel>
      {example.lastResponseId.byteLength > 0 && (
        <>
          <PanelResizeHandle direction='vertical' />
          <Panel id='response' order={2} defaultSize={40}>
            <ResponsePanelLoader responseId={example.lastResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};

interface ResponsePanelLoaderProps {
  responseId: Response['responseId'];
}

const ResponsePanelLoader = ({ responseId }: ResponsePanelLoaderProps) => {
  const responseGetQuery = useConnectQuery(responseGet, { responseId });
  if (!responseGetQuery.isSuccess) return null;
  return <ResponsePanel response={responseGetQuery.data} />;
};

interface ResponsePanelProps {
  response: ResponseGetResponse;
}

const ResponsePanel = ({ response }: ResponsePanelProps) => {
  const { responseId } = response;

  return (
    <Tabs className={tw`flex h-full flex-col`}>
      <div className={tw`flex items-center gap-3 border-b border-slate-200 px-4 text-md`}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            id='body'
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
            id='asserts'
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
        </div>
      </div>

      <div className='flex-1 overflow-auto'>
        <TabPanel id='body' className='flex h-full flex-col gap-4 p-4'>
          <ResponseBodyView bodyBytes={response.body} />
        </TabPanel>

        <TabPanel id='headers' className='p-4'>
          <ResponseHeaderTableLoader responseId={responseId} />
        </TabPanel>

        <TabPanel id='asserts' className='p-4'>
          <ResponseAssertsTableLoader responseId={responseId} />
        </TabPanel>
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

interface ResponseHeaderTableLoaderProps {
  responseId: Response['responseId'];
}

const ResponseHeaderTableLoader = ({ responseId }: ResponseHeaderTableLoaderProps) => {
  const responseHeaderListQuery = useConnectQuery(responseHeaderList, { responseId });
  if (!responseHeaderListQuery.isSuccess) return null;
  return <ResponseHeadersTable headers={responseHeaderListQuery.data.items} />;
};

interface ResponseHeadersTableProps {
  headers: ResponseHeaderListItem[];
}

const ResponseHeadersTable = ({ headers }: ResponseHeadersTableProps) => {
  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeaderListItem>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: headers,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} cellClassName={tw`px-5 py-1.5`} />;
};

interface ResponseAssertsTableLoaderProps {
  responseId: Response['responseId'];
}

const ResponseAssertsTableLoader = ({ responseId }: ResponseAssertsTableLoaderProps) => {
  const responseAssertListQuery = useConnectQuery(responseAssertList, { responseId });
  if (!responseAssertListQuery.isSuccess) return null;
  return <ResponseAssertsTable asserts={responseAssertListQuery.data.items} />;
};

interface ResponseAssertsTableProps {
  asserts: ResponseAssertListItem[];
}

const ResponseAssertsTable = ({ asserts }: ResponseAssertsTableProps) => (
  <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
    {asserts.map(({ assert, result }) => {
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
