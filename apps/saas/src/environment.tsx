import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, Struct } from 'effect';
import { useCallback, useMemo } from 'react';
import { Collection, Dialog, DialogTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuBraces, LuClipboardList, LuPlus, LuTrash2, LuX } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import { Environment, EnvironmentType } from '@the-dev-tools/protobuf/environment/v1/environment_pb';
import {
  createEnvironment,
  getEnvironments,
} from '@the-dev-tools/protobuf/environment/v1/environment-EnvironmentService_connectquery';
import { Variable } from '@the-dev-tools/protobuf/variable/v1/variable_pb';
import {
  createVariable,
  deleteVariable,
  getVariables,
  updateVariable,
} from '@the-dev-tools/protobuf/variable/v1/variable-VariableService_connectquery';
import { Workspace } from '@the-dev-tools/protobuf/workspace/v1/workspace_pb';
import {
  getWorkspace,
  updateWorkspace,
} from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DropdownItem } from '@the-dev-tools/ui/dropdown';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const EnvironmentsWidget = () => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useParams();

  const workspaceQuery = useConnectQuery(getWorkspace, { id: workspaceId });
  const updateWorkspaceMutation = useConnectMutation(updateWorkspace);

  const environmentsQuery = useConnectQuery(getEnvironments, { workspaceId });
  const createEnvironmentMutation = useConnectMutation(createEnvironment);

  if (!environmentsQuery.isSuccess || !workspaceQuery.isSuccess) return null;

  const { environments } = environmentsQuery.data;
  const { workspace } = workspaceQuery.data;

  return (
    <div className='flex justify-between border-b border-black p-2'>
      <Select
        aria-label='Environment'
        selectedKey={workspace?.environment?.id ?? null}
        onSelectionChange={async (key) => {
          const environment = environments.find(({ id }) => id === key);
          if (!environment) return;

          await updateWorkspaceMutation.mutateAsync({ id: workspaceId, envId: environment.id });

          queryClient.setQueryData(
            createConnectQueryKey(getWorkspace, { id: workspaceId }),
            createProtobufSafeUpdater(getWorkspace, (_) => ({
              workspace: new Workspace({ ..._?.workspace, environment }),
            })),
          );
        }}
        triggerClassName={tw`justify-start`}
        triggerVariant='placeholder ghost'
        listBoxItems={environments}
      >
        {(item) => (
          <DropdownItem id={item.id} textValue={item.name}>
            <div className='flex items-center gap-2 text-sm'>
              <div className='flex size-7 items-center justify-center rounded-md border border-black bg-neutral-200'>
                {item.type === EnvironmentType.GLOBAL ? <LuBraces /> : item.name[0]}
              </div>
              <span className='font-semibold'>
                {item.type === EnvironmentType.GLOBAL ? 'Global Environment' : item.name}
              </span>
            </div>
          </DropdownItem>
        )}
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
                        const environment = new Environment({ name: 'New Environment' });
                        const { id } = await createEnvironmentMutation.mutateAsync({ workspaceId, environment });
                        environment.id = id;

                        queryClient.setQueryData(
                          createConnectQueryKey(getEnvironments, { workspaceId }),
                          createProtobufSafeUpdater(getEnvironments, (_) =>
                            Struct.evolve(_!, { environments: (_) => Array.append(_, environment) }),
                          ),
                        );
                      }}
                    >
                      <LuPlus />
                    </Button>
                  </div>

                  <TabList className='contents' items={environments}>
                    {(item) => (
                      <Tab
                        id={item.id}
                        className={({ isSelected }) =>
                          twJoin(
                            tw`-m-1 flex cursor-pointer items-center gap-2 rounded p-1 text-sm`,
                            isSelected && tw`bg-neutral-400`,
                            item.type === EnvironmentType.GLOBAL && tw`-order-2`,
                          )
                        }
                      >
                        <div className='flex size-6 items-center justify-center rounded bg-neutral-400 p-1'>
                          {item.type === EnvironmentType.GLOBAL ? <LuBraces /> : item.name[0]}
                        </div>
                        <span>{item.type === EnvironmentType.GLOBAL ? 'Global Variables' : item.name}</span>
                      </Tab>
                    )}
                  </TabList>
                </div>

                <Collection items={environments}>
                  {(item) => (
                    <TabPanel id={item.id} className='flex h-full min-w-0 flex-1 flex-col'>
                      <div className='px-6 py-4'>
                        <div className='mb-4 flex items-start'>
                          <div className='flex-1'>
                            <h1 className='text-xl font-medium'>
                              {item.type === EnvironmentType.GLOBAL ? 'Global Variables' : item.name}
                            </h1>
                            {item.description && <span className='text-sm font-light'>{item.description}</span>}
                          </div>

                          <Button variant='placeholder ghost' kind='placeholder' onPress={close}>
                            <LuX />
                          </Button>
                        </div>

                        <VariablesTableLoader environmentId={item.id} />
                      </div>

                      <div className='flex-1' />

                      <div className='flex justify-end border-t border-black bg-neutral-100 px-6 py-4'>
                        <Button kind='placeholder' variant='placeholder' onPress={close}>
                          Save
                        </Button>
                      </div>
                    </TabPanel>
                  )}
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
  environmentId: string;
}

const VariablesTableLoader = ({ environmentId }: VariablesTableLoaderProps) => {
  const { data, isSuccess } = useConnectQuery(getVariables, { environmentId });
  if (!isSuccess) return;
  return <VariablesTable environmentId={environmentId} variables={data.variables} />;
};

interface VariablesTableProps extends VariablesTableLoaderProps {
  variables: Variable[];
}

const VariablesTable = ({ environmentId, variables }: VariablesTableProps) => {
  const createMutation = useConnectMutation(createVariable);
  const updateMutation = useConnectMutation(updateVariable);
  const { mutate: deleteMutate } = useConnectMutation(deleteVariable);

  const makeItem = useCallback((item?: Partial<Variable>) => new Variable({ ...item, enabled: true }), []);

  const values = useMemo(() => ({ items: [...variables, makeItem()] }), [makeItem, variables]);
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({ control: form.control, name: 'items' });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<Variable>();
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
                deleteMutate({ id: getValues(`items.${row.index}.id`) });
                removeField(row.index);
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, deleteMutate, getValues, removeField]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: Struct.get('id'),
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    onCreate: async (variable) => (await createMutation.mutateAsync({ environmentId, variable })).id,
    onUpdate: (variable) => updateMutation.mutateAsync({ variable }),
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
