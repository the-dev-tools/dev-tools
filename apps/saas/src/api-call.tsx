import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Link, Outlet } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, HashMap, Match, MutableHashMap, Option, pipe } from 'effect';
import { format as prettierFormat } from 'prettier/standalone';
import { useMemo, useState } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { LuSave, LuSendHorizonal } from 'react-icons/lu';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twMerge } from 'tailwind-merge';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall, updateApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { ApiExampleResponse, Query, ResponseHeader } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createQuery,
  runExample,
  updateQuery,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select, SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId')({
  component: Page,
});

function Page() {
  const { apiCallId, exampleId } = Route.useParams();

  const query = useConnectQuery(getApiCall, { id: apiCallId, exampleId });

  if (!query.isSuccess) return null;
  const { data } = query;

  return <ApiForm data={data} />;
}

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

class ApiFormData extends Schema.Class<ApiFormData>('ApiCallFormData')({
  method: Schema.String.pipe(Schema.filter((_) => Array.contains(methods, _) || 'Method is not valid')),
  url: Schema.String.pipe(Schema.nonEmptyString({ message: () => 'URL must not be empty' })),
}) {}

interface ApiFormProps {
  data: GetApiCallResponse;
}

const ApiForm = ({ data }: ApiFormProps) => {
  const { apiCallId, exampleId } = Route.useParams();

  const queryClient = useQueryClient();

  const updateMutation = useConnectMutation(updateApiCall);
  const runMutation = useConnectMutation(runExample);

  const updateQueryMutation = useConnectMutation(updateQuery);
  const createQueryMutation = useConnectMutation(createQuery);

  const values = useMemo(() => {
    const { origin, pathname } = new URL(data.apiCall!.url);
    const url = pipe(
      data.example!.query,
      Array.filterMap((_) => {
        if (!_.enabled) return Option.none();
        else return Option.some([_.key, _.value]);
      }),
      (_) => new URLSearchParams(_).toString(),
      (_) => origin + pathname + '?' + _,
    );
    return new ApiFormData({
      url,
      method: data.apiCall!.meta!.method,
    });
  }, [data.apiCall, data.example]);

  const form = useForm({
    resolver: effectTsResolver(ApiFormData),
    values,
  });

  const onSubmit = form.handleSubmit(async (formData) => {
    const { origin, pathname, searchParams } = new URL(formData.url);

    updateMutation.mutate({
      apiCall: {
        ...data.apiCall,
        url: origin + pathname,
        meta: { ...data.apiCall?.meta, method: formData.method },
      },
    });

    const queryMap = pipe(
      searchParams.entries(),
      Array.fromIterable,
      Array.map(([key, value]) => [key + value, new Query({ key, value, enabled: true, exampleId })] as const),
      MutableHashMap.fromIterable,
    );

    data.example!.query.forEach((query) => {
      MutableHashMap.modifyAt(
        queryMap,
        query.key + query.value,
        Option.match({
          onSome: () => {
            if (query.enabled) return Option.none();
            else return Option.some(new Query({ ...query, enabled: true }));
          },
          onNone: () => {
            if (!query.enabled) return Option.none();
            return Option.some(new Query({ ...query, exampleId, enabled: false }));
          },
        }),
      );
    });

    const queryIdIndexMap = pipe(
      data.example!.query,
      Array.map(({ id }, index) => [id, index] as const),
      HashMap.fromIterable,
    );

    const newQueryList = [...data.example!.query];
    await pipe(
      Array.fromIterable(queryMap),
      Array.map(async ([_, query]) => {
        if (query.id) {
          await updateQueryMutation.mutateAsync({ query });
          const index = HashMap.unsafeGet(queryIdIndexMap, query.id);
          newQueryList[index] = query;
        } else {
          const { id } = await createQueryMutation.mutateAsync({ query });
          newQueryList.push(new Query({ ...query, id }));
        }
      }),
      (_) => Promise.allSettled(_),
    );

    queryClient.setQueryData(
      createConnectQueryKey(getApiCall, { id: apiCallId, exampleId }),
      createProtobufSafeUpdater(getApiCall, (old) => ({ ...old, example: { ...old?.example, query: newQueryList } })),
    );
  });

  return (
    <PanelGroup direction='vertical'>
      <Panel id='request' order={1} className='flex h-full flex-col'>
        <form onSubmit={onSubmit} onBlur={onSubmit}>
          <div className='flex items-center gap-2 border-b-2 border-black px-4 py-3'>
            <h2 className='flex-1 truncate text-sm font-bold'>{data.apiCall!.meta!.name}</h2>

            <Button kind='placeholder' variant='placeholder' type='submit'>
              <LuSave /> Save
            </Button>
          </div>

          <div className='flex items-start p-4 pb-0'>
            <SelectRHF
              control={form.control}
              name='method'
              aria-label='Method'
              triggerClassName={tw`rounded-r-none border-r-0`}
            >
              {methods.map((_) => (
                <DropdownItem key={_} id={_}>
                  {_}
                </DropdownItem>
              ))}
            </SelectRHF>

            <TextFieldRHF
              control={form.control}
              name='url'
              aria-label='URL'
              className={tw`flex-1`}
              inputClassName={tw`rounded-none border-x-0 bg-neutral-200`}
            />

            {/* TODO: implement */}
            <Button
              kind='placeholder'
              variant='placeholder'
              className='rounded-l-none border-l-0 bg-black text-white'
              onPress={async () => {
                await onSubmit();
                const { response } = await runMutation.mutateAsync({ id: exampleId });
                if (!response) return;
                queryClient.setQueryData(
                  createConnectQueryKey(getApiCall, { id: apiCallId, exampleId }),
                  createProtobufSafeUpdater(getApiCall, (old) => ({ ...old, example: { ...old?.example, response } })),
                );
              }}
            >
              Send <LuSendHorizonal className='size-4' />
            </Button>
          </div>
        </form>

        <div className='flex flex-1 flex-col gap-4 overflow-auto p-4'>
          <div className='flex gap-4 border-b border-black'>
            <Link
              className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
              activeProps={{ className: tw`border-b-black` }}
              activeOptions={{ exact: true }}
              from='/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId'
              to='.'
            >
              Params
            </Link>
            <Link
              className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
              activeProps={{ className: tw`border-b-black` }}
              from='/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId'
              to='headers'
            >
              Headers
            </Link>
            <Link
              className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
              activeProps={{ className: tw`border-b-black` }}
              from='/workspace/$workspaceId/api-call/$apiCallId/example/$exampleId'
              to='body'
            >
              Body
            </Link>
          </div>

          <Outlet />
        </div>
      </Panel>
      {data.example?.response && (
        <>
          <PanelResizeHandle direction='vertical' />
          <Panel id='response' order={2} defaultSize={40}>
            <ResponsePanel response={data.example.response} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};

interface ResponsePanelProps {
  response: ApiExampleResponse;
}

const ResponsePanel = ({ response }: ResponsePanelProps) => {
  return (
    <Tabs className='flex h-full flex-col'>
      <div className='flex items-center gap-2 border-b border-black pl-2 pr-4 text-sm text-neutral-500'>
        <TabList className='contents'>
          <Tab
            id='body'
            className={({ isSelected }) =>
              twMerge(
                tw`cursor-pointer border-b-2 border-transparent p-2 transition-colors`,
                isSelected && tw`border-black text-black`,
              )
            }
          >
            Body
          </Tab>
          <Tab
            id='headers'
            className={({ isSelected }) =>
              twMerge(
                tw`cursor-pointer border-b-2 border-transparent p-2 transition-colors`,
                isSelected && tw`border-black text-black`,
              )
            }
          >
            Headers
          </Tab>
        </TabList>

        <div className='flex-1' />

        <div>
          Status: <span className='text-black'>{response.status}</span>
        </div>

        <div>
          Time: <span className='text-black'>{pipe(Number(response.duration), Duration.millis, Duration.format)}</span>
        </div>
      </div>

      <div className='flex-1 overflow-auto'>
        <TabPanel id='body' className='flex h-full flex-col gap-4 p-4'>
          <ResponseBodyView bodyBytes={response.body} />
        </TabPanel>

        <TabPanel id='headers' className='p-4'>
          <ResponseHeadersTable headers={response.headers} />
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
      <TabList className='flex gap-2 self-start rounded bg-neutral-400 p-1 text-sm'>
        <Tab
          className={({ isSelected }) => twMerge(tw`cursor-pointer rounded px-2 py-1`, isSelected && tw`bg-white`)}
          id='pretty'
        >
          Pretty
        </Tab>
        <Tab
          className={({ isSelected }) => twMerge(tw`cursor-pointer rounded px-2 py-1`, isSelected && tw`bg-white`)}
          id='raw'
        >
          Raw
        </Tab>
        <Tab
          className={({ isSelected }) => twMerge(tw`cursor-pointer rounded px-2 py-1`, isSelected && tw`bg-white`)}
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
        triggerClassName={tw`px-1.5 py-1`}
        selectedKey={language}
        onSelectionChange={(_) => void setLanguage(_ as (typeof languages)[number])}
      >
        {languages.map((_) => (
          <DropdownItem key={_} id={_}>
            {_}
          </DropdownItem>
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

interface ResponseHeadersTableProps {
  headers: ResponseHeader[];
}

const ResponseHeadersTable = ({ headers }: ResponseHeadersTableProps) => {
  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeader>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: headers,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className='rounded border border-black'>
      <table className='w-full divide-inherit border-inherit'>
        <thead className='divide-y divide-inherit border-b border-inherit'>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className='p-1.5 text-left text-sm font-normal capitalize text-neutral-500'
                  style={{ width: ((header.getSize() / table.getTotalSize()) * 100).toString() + '%' }}
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className='divide-y divide-inherit'>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className='break-all p-1 align-middle text-sm'>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
