import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, Link, Outlet } from '@tanstack/react-router';
import { Array, HashMap, MutableHashMap, Option, pipe } from 'effect';
import { useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { LuSave, LuSendHorizonal } from 'react-icons/lu';

import { GetApiCallResponse } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import { getApiCall, updateApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Query } from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample_pb';
import {
  createQuery,
  updateQuery,
} from '@the-dev-tools/protobuf/itemapiexample/v1/itemapiexample-ItemApiExampleService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId')({
  component: Page,
});

function Page() {
  const { apiCallId } = Route.useParams();

  const query = useConnectQuery(getApiCall, { id: apiCallId });

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
  const { workspaceId, apiCallId } = Route.useParams();

  const queryClient = useQueryClient();

  const updateMutation = useConnectMutation(updateApiCall);

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
      Array.map(
        ([key, value]) =>
          [
            key + value,
            new Query({
              key,
              value,
              enabled: true,
              exampleId: data.example!.meta!.id,
            }),
          ] as const,
      ),
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
            return Option.some(new Query({ ...query, exampleId: data.example!.meta!.id, enabled: false }));
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
      createConnectQueryKey(getApiCall, { id: apiCallId }),
      createProtobufSafeUpdater(getApiCall, (_) => ({
        ..._,
        example: {
          ..._!.example,
          query: newQueryList,
        },
      })),
    );
  });

  return (
    <div className='flex h-full flex-col'>
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
          <Button kind='placeholder' variant='placeholder' className='rounded-l-none border-l-0 bg-black text-white'>
            Send <LuSendHorizonal className='size-4' />
          </Button>
        </div>
      </form>

      <div className='flex flex-1 flex-col gap-4 p-4'>
        <div className='flex gap-4 border-b border-black'>
          <Link
            className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
            activeProps={{ className: tw`border-b-black` }}
            activeOptions={{ exact: true }}
            from='/workspace/$workspaceId/api-call/$apiCallId'
            to='.'
          >
            Params
          </Link>
          <Link
            className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
            activeProps={{ className: tw`border-b-black` }}
            from='/workspace/$workspaceId/api-call/$apiCallId'
            to='headers'
            params={{ workspaceId, apiCallId }}
          >
            Headers
          </Link>
          <Link
            className={tw`border-b-2 border-transparent p-1 text-sm transition-colors`}
            activeProps={{ className: tw`border-b-black` }}
            from='/workspace/$workspaceId/api-call/$apiCallId'
            to='body'
            params={{ workspaceId, apiCallId }}
          >
            Body
          </Link>
        </div>

        <Outlet />
      </div>
    </div>
  );
};
