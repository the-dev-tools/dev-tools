import { create, fromJson, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, pipe } from 'effect';
import { Ulid } from 'id128';
import { useCallback, useMemo } from 'react';
import { Collection, Dialog, DialogTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuBraces, LuClipboardList, LuPlus, LuTrash2, LuX } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import {
  Environment,
  EnvironmentJson,
  EnvironmentListItemSchema,
  EnvironmentListResponseSchema,
} from '@the-dev-tools/spec/environment/v1/environment_pb';
import {
  environmentCreate,
  environmentList,
} from '@the-dev-tools/spec/environment/v1/environment-EnvironmentService_connectquery';
import {
  VariableCreateResponseSchema,
  VariableJson,
  VariableListItem,
  VariableListItemSchema,
  VariableListResponseSchema,
  VariableSchema,
  VariableUpdateRequestSchema,
} from '@the-dev-tools/spec/variable/v1/variable_pb';
import {
  variableCreate,
  variableDelete,
  variableList,
  variableUpdate,
} from '@the-dev-tools/spec/variable/v1/variable-VariableService_connectquery';
import {
  workspaceGet,
  workspaceUpdate,
} from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const EnvironmentsWidget = () => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const workspaceQuery = useConnectQuery(workspaceGet, { workspaceId });
  const updateWorkspaceMutation = useConnectMutation(workspaceUpdate);

  const environmentsQuery = useConnectQuery(environmentList, { workspaceId });
  const createEnvironmentMutation = useConnectMutation(environmentCreate);

  if (!environmentsQuery.isSuccess || !workspaceQuery.isSuccess) return null;

  const environments = environmentsQuery.data.items;
  const { selectedEnvironmentId } = workspaceQuery.data;
  const selectedEnvironmentIdCan = Ulid.construct(selectedEnvironmentId).toCanonical();

  return (
    <div className='flex justify-between border-b border-black p-2'>
      <Select
        aria-label='Environment'
        selectedKey={selectedEnvironmentIdCan}
        onSelectionChange={async (selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          await updateWorkspaceMutation.mutateAsync({ workspaceId, selectedEnvironmentId });

          queryClient.setQueryData(
            createConnectQueryKey({ schema: workspaceGet, cardinality: 'finite', input: { workspaceId } }),
            createProtobufSafeUpdater(workspaceGet, (old) => {
              if (old === undefined) return undefined;
              return { ...old, selectedEnvironmentId };
            }),
          );
        }}
        triggerClassName={tw`justify-start`}
        triggerVariant='placeholder ghost'
        listBoxItems={environments}
      >
        {(item) => {
          const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
          return (
            <DropdownItem id={environmentIdCan} textValue={item.name}>
              <div className='flex items-center gap-2 text-sm'>
                <div className='flex size-7 items-center justify-center rounded-md border border-black bg-neutral-200'>
                  {item.isGlobal ? <LuBraces /> : item.name[0]}
                </div>
                <span className='font-semibold'>{item.isGlobal ? 'Global Environment' : item.name}</span>
              </div>
            </DropdownItem>
          );
        }}
      </Select>

      <DialogTrigger>
        <Button kind='placeholder' variant='placeholder ghost' className='aspect-square'>
          <LuClipboardList />
        </Button>

        <Modal modalClassName={tw`size-full`}>
          <Dialog className='h-full outline-none'>
            {({ close }) => (
              <Tabs className='flex h-full'>
                <div className='flex w-72 flex-col gap-2 border-r border-black bg-neutral-200 p-4'>
                  <div className='-order-3 mb-2'>
                    <div className='text-lg font-medium'>Variable Settings</div>
                    <span className='text-sm font-light'>Manage variables & environment</span>
                  </div>

                  <div className='-order-1 my-1 flex items-center justify-between'>
                    <span className='text-neutral-600'>Environments</span>

                    <Button
                      kind='placeholder'
                      variant='placeholder'
                      className='p-1'
                      onPress={async () => {
                        const environment = { name: 'New Environment' } satisfies EnvironmentJson;
                        const { environmentId } = await createEnvironmentMutation.mutateAsync({
                          ...environment,
                          workspaceId,
                        });

                        queryClient.setQueryData(
                          createConnectQueryKey({
                            schema: environmentList,
                            cardinality: 'finite',
                            input: { workspaceId },
                          }),
                          createProtobufSafeUpdater(environmentList, (old) =>
                            create(EnvironmentListResponseSchema, {
                              items: Array.append(
                                old?.items ?? [],
                                create(EnvironmentListItemSchema, { ...environment, environmentId }),
                              ),
                            }),
                          ),
                        );
                      }}
                    >
                      <LuPlus />
                    </Button>
                  </div>

                  <TabList className='contents' items={environments}>
                    {(item) => {
                      const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
                      return (
                        <Tab
                          id={environmentIdCan}
                          className={({ isSelected }) =>
                            twJoin(
                              tw`-m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                              isSelected && tw`bg-neutral-400`,
                              item.isGlobal && tw`-order-2`,
                            )
                          }
                        >
                          <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>
                            {item.isGlobal ? <LuBraces /> : item.name[0]}
                          </div>
                          <span>{item.isGlobal ? 'Global Variables' : item.name}</span>
                        </Tab>
                      );
                    }}
                  </TabList>
                </div>

                <Collection items={environments}>
                  {(item) => {
                    const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
                    return (
                      <TabPanel id={environmentIdCan} className='flex h-full min-w-0 flex-1 flex-col'>
                        <div className='px-6 py-4'>
                          <div className='mb-4 flex items-start'>
                            <div className='flex-1'>
                              <h1 className='text-xl font-medium'>{item.isGlobal ? 'Global Variables' : item.name}</h1>
                              {item.description && <span className='text-sm font-light'>{item.description}</span>}
                            </div>

                            <Button variant='placeholder ghost' kind='placeholder' onPress={close}>
                              <LuX />
                            </Button>
                          </div>

                          <VariablesTableLoader environmentId={item.environmentId} />
                        </div>

                        <div className='flex-1' />

                        <div className='flex justify-end border-t border-black bg-neutral-100 px-6 py-4'>
                          <Button kind='placeholder' variant='placeholder' onPress={close}>
                            Save
                          </Button>
                        </div>
                      </TabPanel>
                    );
                  }}
                </Collection>
              </Tabs>
            )}
          </Dialog>
        </Modal>
      </DialogTrigger>
    </div>
  );
};

interface VariablesTableLoaderProps {
  environmentId: Environment['environmentId'];
}

const VariablesTableLoader = ({ environmentId }: VariablesTableLoaderProps) => {
  const { data, isSuccess } = useConnectQuery(variableList, { environmentId });
  if (!isSuccess) return;
  return <VariablesTable environmentId={environmentId} items={data.items} />;
};

interface VariablesTableProps extends VariablesTableLoaderProps {
  items: VariableListItem[];
}

const VariablesTable = ({ environmentId, items }: VariablesTableProps) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const createMutation = useConnectMutation(variableCreate);
  const updateMutation = useConnectMutation(variableUpdate);
  const { mutate: deleteMutate } = useConnectMutation(variableDelete);

  const makeItem = useCallback(
    (variableId?: string, item?: VariableJson) => ({ ...item, variableId: variableId ?? '', enabled: true }),
    [],
  );
  const values = useMemo(
    () => ({ items: [...items.map((_): VariableJson => toJson(VariableListItemSchema, _)), makeItem()] }),
    [items, makeItem],
  );
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({
    control: form.control,
    name: 'items',
    keyName: 'variableId',
  });

  const onChange = useCallback(
    () => queryClient.invalidateQueries(createQueryOptions(variableList, { workspaceId }, { transport })),
    [queryClient, transport, workspaceId],
  );

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<VariableJson>();
    return [
      accessor('enabled', {
        header: '',
        size: 0,
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table}>
            <CheckboxRHF control={form.control} name={`items.${row.index}.enabled`} variant='table-cell' />
          </HidePlaceholderCell>
        ),
      }),
      accessor('name', {
        cell: ({ row }) => (
          <TextFieldRHF control={form.control} name={`items.${row.index}.name`} variant='table-cell' />
        ),
      }),
      accessor('value', {
        cell: ({ row: { index } }) => (
          <TextFieldRHF control={form.control} name={`items.${index}.value`} variant='table-cell' />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF control={form.control} name={`items.${row.index}.description`} variant='table-cell' />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        size: 0,
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table}>
            <Button
              className='text-red-700'
              kind='placeholder'
              variant='placeholder ghost'
              onPress={() => {
                const variableIdJson = getValues(`items.${row.index}.variableId`);
                if (variableIdJson === undefined) return;
                const { variableId } = fromJson(VariableSchema, { variableId: variableIdJson });
                deleteMutate({ variableId });
                removeField(row.index);
                void onChange();
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, deleteMutate, getValues, removeField, onChange]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.variableId ?? '',
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(async () => {
    await onChange();
    const items = pipe(
      getValues('items'),
      Array.dropRight(1),
      Array.map((_) => fromJson(VariableListItemSchema, _)),
    );
    queryClient.setQueryData(
      createConnectQueryKey({ schema: variableList, cardinality: 'finite', input: { items } }),
      createProtobufSafeUpdater(variableList, () => create(VariableListResponseSchema, { items })),
    );
  }, [getValues, onChange, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    getRowId: (_) => _.variableId,
    onCreate: async (variable) => {
      const response = await createMutation.mutateAsync({ ...variable, environmentId });
      return toJson(VariableCreateResponseSchema, response).variableId ?? '';
    },
    onUpdate: (variable) => updateMutation.mutateAsync(fromJson(VariableUpdateRequestSchema, variable)),
    onChange,
    setData,
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
