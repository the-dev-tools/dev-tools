import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Ulid } from 'id128';
import { Suspense } from 'react';
import { Collection, Dialog, DialogTrigger, MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';

import { EnvironmentListItem } from '@the-dev-tools/spec/environment/v1/environment_pb';
import {
  environmentCreate,
  environmentDelete,
  environmentList,
  environmentUpdate,
} from '@the-dev-tools/spec/environment/v1/environment-EnvironmentService_connectquery';
import { VariableListItem } from '@the-dev-tools/spec/variable/v1/variable_pb';
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
import { DataTable } from '@the-dev-tools/ui/data-table';
import { GlobalEnvironmentIcon, Spinner, VariableIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useConnectMutation, useConnectSuspenseQuery } from '~/api/connect-query';

import {
  ColumnActionDelete,
  columnActions,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
} from './form-table';
import { ImportDialog } from './workspace/import';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const EnvironmentsWidget = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { transport } = workspaceRoute.useRouteContext();

  const [workspaceGetQuery, environmentListQuery] = useSuspenseQueries({
    queries: [
      createQueryOptions(workspaceGet, { workspaceId }, { transport }),
      createQueryOptions(environmentList, { workspaceId }, { transport }),
    ],
  });

  const workspaceUpdateMutation = useConnectMutation(workspaceUpdate);
  const environmentCreateMutation = useConnectMutation(environmentCreate);

  const environments = environmentListQuery.data.items;
  const { selectedEnvironmentId } = workspaceGetQuery.data;
  const selectedEnvironmentIdCan = Ulid.construct(selectedEnvironmentId).toCanonical();

  return (
    <div className={tw`flex gap-1 border-b border-slate-200 p-3`}>
      <Select
        aria-label='Environment'
        listBoxItems={environments}
        onSelectionChange={(selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          workspaceUpdateMutation.mutate({ selectedEnvironmentId, workspaceId });
        }}
        selectedKey={selectedEnvironmentIdCan}
        triggerClassName={tw`justify-start p-0`}
        triggerVariant='ghost'
      >
        {(item) => {
          const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
          return (
            <ListBoxItem id={environmentIdCan} textValue={item.name}>
              <div className={tw`flex items-center gap-2`}>
                <div
                  className={tw`flex size-6 items-center justify-center rounded-md bg-slate-200 text-xs text-slate-500`}
                >
                  {item.isGlobal ? <VariableIcon /> : item.name[0]}
                </div>
                <span className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>
                  {item.isGlobal ? 'Global Environment' : item.name}
                </span>
              </div>
            </ListBoxItem>
          );
        }}
      </Select>

      <div className={tw`flex-1`} />

      <ImportDialog />

      <DialogTrigger>
        <Button className={tw`p-1`} variant='ghost'>
          <GlobalEnvironmentIcon className={tw`size-4 text-slate-500`} />
        </Button>

        <Modal>
          <Dialog className={tw`outline-hidden h-full`}>
            {({ close }) => (
              <Tabs className={tw`flex h-full`} orientation='vertical'>
                <div className={tw`flex w-64 flex-col border-r border-slate-200 bg-slate-50 p-4 tracking-tight`}>
                  <div className={tw`-order-3 mb-4`}>
                    <div className={tw`mb-0.5 text-sm font-semibold leading-5 text-slate-800`}>Variable Settings</div>
                    <div className={tw`text-xs leading-4 text-slate-500`}>Manage variables & environment</div>
                  </div>

                  <div className={tw`-order-1 mb-1 mt-3 flex items-center justify-between py-0.5`}>
                    <span className={tw`text-md leading-5 text-slate-400`}>Environments</span>

                    <Button
                      className={tw`bg-slate-200 p-0.5`}
                      onPress={() => void environmentCreateMutation.mutate({ name: 'New Environment', workspaceId })}
                      variant='ghost'
                    >
                      <FiPlus className={tw`size-4 text-slate-500`} />
                    </Button>
                  </div>

                  <TabList className={tw`contents`} items={environments}>
                    {(item) => {
                      const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
                      return (
                        <Tab
                          className={({ isSelected }) =>
                            twJoin(
                              tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                              isSelected && tw`bg-slate-200`,
                              item.isGlobal && tw`-order-2`,
                            )
                          }
                          id={environmentIdCan}
                        >
                          {item.isGlobal ? (
                            <VariableIcon className={tw`size-4 text-slate-500`} />
                          ) : (
                            <div
                              className={tw`flex size-4 items-center justify-center rounded-sm bg-slate-300 text-xs leading-3 text-slate-500`}
                            >
                              {item.name[0]}
                            </div>
                          )}
                          <span className={tw`text-md font-semibold leading-5`}>
                            {item.isGlobal ? 'Global Variables' : item.name}
                          </span>
                        </Tab>
                      );
                    }}
                  </TabList>
                </div>

                <div className={tw`flex h-full min-w-0 flex-1 flex-col`}>
                  <Collection items={environments}>
                    {(_) => {
                      const id = Ulid.construct(_.environmentId).toCanonical();
                      return <EnvironmentPanel environment={_} id={id} />;
                    }}
                  </Collection>

                  <div className={tw`flex-1`} />

                  <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
                    <Button onPress={close} variant='primary'>
                      Close
                    </Button>
                  </div>
                </div>
              </Tabs>
            )}
          </Dialog>
        </Modal>
      </DialogTrigger>
    </div>
  );
};

interface EnvironmentPanelProps {
  environment: EnvironmentListItem;
  id: string;
}

const EnvironmentPanel = ({ environment: { environmentId, isGlobal, name }, id }: EnvironmentPanelProps) => {
  const environmentUpdateMutation = useConnectMutation(environmentUpdate);
  const environmentDeleteMutation = useConnectMutation(environmentDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => environmentUpdateMutation.mutateAsync({ environmentId, name: _ }),
    value: name,
  });

  return (
    <TabPanel className={tw`h-full px-6 py-4`} id={id}>
      <div className={tw`mb-4 flex items-center gap-2`} onContextMenu={onContextMenu}>
        {isGlobal ? (
          <VariableIcon className={tw`size-6 text-slate-500`} />
        ) : (
          <div
            className={tw`flex size-6 items-center justify-center rounded-md bg-slate-300 text-xs leading-3 text-slate-500`}
          >
            {name[0]}
          </div>
        )}

        {isEditing ? (
          <TextField
            inputClassName={tw`-my-1 py-1 font-semibold leading-none tracking-tight text-slate-800`}
            isDisabled={environmentUpdateMutation.isPending}
            {...textFieldProps}
          />
        ) : (
          <h1 className={tw`font-semibold leading-5 tracking-tight text-slate-800`}>
            {isGlobal ? 'Global Variables' : name}
          </h1>
        )}

        <div className={tw`flex-1`} />

        {!isGlobal && (
          <MenuTrigger {...menuTriggerProps}>
            <Button className={tw`p-1`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem onAction={() => void environmentDeleteMutation.mutate({ environmentId })} variant='danger'>
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner className={tw`size-12`} />
          </div>
        }
      >
        <VariablesTable environmentId={environmentId} />
      </Suspense>
    </TabPanel>
  );
};

interface VariablesTableProps {
  environmentId: Uint8Array;
}

export const VariablesTable = ({ environmentId }: VariablesTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(variableList, { environmentId });

  const { mutateAsync: create } = useConnectMutation(variableCreate);
  const { mutateAsync: update } = useConnectMutation(variableUpdate);

  const table = useReactTable({
    columns: [
      columnCheckboxField<VariableListItem>('enabled', { meta: { divider: false } }),
      columnReferenceField<VariableListItem>('name'),
      columnReferenceField<VariableListItem>('value'),
      columnTextField<VariableListItem>('description', { meta: { divider: false } }),
      columnActions<VariableListItem>({
        cell: ({ row }) => (
          <ColumnActionDelete input={{ variableId: row.original.variableId }} schema={variableDelete} />
        ),
      }),
    ],
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  const formTable = useFormTable({
    createLabel: 'New variable',
    items,
    onCreate: () => create({ enabled: true, environmentId, name: `VARIABLE_${items.length}` }),
    onUpdate: ({ $typeName: _, ...item }) => update(item),
    primaryColumn: 'name',
  });

  return <DataTable {...formTable} table={table} />;
};
