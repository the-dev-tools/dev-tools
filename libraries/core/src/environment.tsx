import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { getRouteApi, useRouteContext } from '@tanstack/react-router';
import { ColumnDef, createColumnHelper } from '@tanstack/react-table';
import { Struct } from 'effect';
import { Ulid } from 'id128';
import { Suspense, useMemo } from 'react';
import { Collection, Dialog, DialogTrigger, MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { EnvironmentListItem } from '@the-dev-tools/spec/environment/v1/environment_pb';
import {
  environmentCreate,
  environmentDelete,
  environmentList,
  environmentUpdate,
} from '@the-dev-tools/spec/environment/v1/environment-EnvironmentService_connectquery';
import { VariableListItem, VariableListItemSchema, VariableService } from '@the-dev-tools/spec/variable/v1/variable_pb';
import { variableList } from '@the-dev-tools/spec/variable/v1/variable-VariableService_connectquery';
import {
  workspaceGet,
  workspaceUpdate,
} from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { GlobalEnvironmentIcon, VariableIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldRHF, useEditableTextState } from '@the-dev-tools/ui/text-field';

import { FormTableItem, genericFormTableActionColumn, genericFormTableEnableColumn, useFormTable } from './form-table';

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
    <div className={tw`flex justify-between border-b border-slate-200 p-3`}>
      <Select
        aria-label='Environment'
        selectedKey={selectedEnvironmentIdCan}
        onSelectionChange={(selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          workspaceUpdateMutation.mutate({ workspaceId, selectedEnvironmentId });
        }}
        triggerClassName={tw`justify-start p-0`}
        triggerVariant='ghost'
        listBoxItems={environments}
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

      <DialogTrigger>
        <Button variant='ghost' className={tw`p-1`}>
          <GlobalEnvironmentIcon className={tw`size-4 text-slate-500`} />
        </Button>

        <Modal modalClassName={tw`size-full`}>
          <Dialog className={tw`h-full outline-none`}>
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
                      variant='ghost'
                      className={tw`bg-slate-200 p-0.5`}
                      onPress={() => void environmentCreateMutation.mutate({ workspaceId, name: 'New Environment' })}
                    >
                      <FiPlus className={tw`size-4 text-slate-500`} />
                    </Button>
                  </div>

                  <TabList className={tw`contents`} items={environments}>
                    {(item) => {
                      const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
                      return (
                        <Tab
                          id={environmentIdCan}
                          className={({ isSelected }) =>
                            twJoin(
                              tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                              isSelected && tw`bg-slate-200`,
                              item.isGlobal && tw`-order-2`,
                            )
                          }
                        >
                          {item.isGlobal ? (
                            <VariableIcon className={tw`size-4 text-slate-500`} />
                          ) : (
                            <div
                              className={tw`flex size-4 items-center justify-center rounded bg-slate-300 text-xs leading-3 text-slate-500`}
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
                      return <EnvironmentPanel id={id} environment={_} />;
                    }}
                  </Collection>

                  <div className={tw`flex-1`} />

                  <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
                    <Button variant='primary' onPress={close}>
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
  id: string;
  environment: EnvironmentListItem;
}

const EnvironmentPanel = ({ id, environment: { environmentId, isGlobal, name } }: EnvironmentPanelProps) => {
  const environmentUpdateMutation = useConnectMutation(environmentUpdate);
  const environmentDeleteMutation = useConnectMutation(environmentDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: name,
    onSuccess: (_) => environmentUpdateMutation.mutateAsync({ environmentId, name: _ }),
  });

  return (
    <TabPanel id={id} className={tw`px-6 py-4`}>
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
            <Button variant='ghost' className={tw`p-1`}>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem variant='danger' onAction={() => void environmentDeleteMutation.mutate({ environmentId })}>
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      <Suspense fallback={'Loading variables...'}>
        <VariablesTable environmentId={environmentId} />
      </Suspense>
    </TabPanel>
  );
};

const variableColumnHelper = createColumnHelper<FormTableItem<VariableListItem>>();

const variableColumns = [
  genericFormTableEnableColumn,
  variableColumnHelper.accessor('data.name', {
    header: 'Name',
    meta: { divider: false },
    cell: ({ table, row }) => (
      <TextFieldRHF control={table.options.meta!.control!} name={`items.${row.index}.data.name`} variant='table-cell' />
    ),
  }),
  variableColumnHelper.accessor('data.value', {
    header: 'Value',
    cell: ({ table, row }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.value`}
        variant='table-cell'
      />
    ),
  }),
  variableColumnHelper.accessor('data.description', {
    header: 'Description',
    cell: ({ table, row }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.description`}
        variant='table-cell'
      />
    ),
  }),
  genericFormTableActionColumn,
];

interface VariablesTableProps {
  environmentId: Uint8Array;
}

export const VariablesTable = ({ environmentId }: VariablesTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const variableService = useMemo(() => createClient(VariableService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(variableList, { environmentId });

  const table = useFormTable({
    items,
    schema: VariableListItemSchema,
    columns: variableColumns as ColumnDef<FormTableItem<VariableListItem>>[],
    onCreate: (_) =>
      variableService.variableCreate({ ...Struct.omit(_, '$typeName'), environmentId }).then((_) => _.variableId),
    onUpdate: (_) => variableService.variableUpdate(Struct.omit(_, '$typeName')),
    onDelete: (_) => variableService.variableDelete(Struct.omit(_, '$typeName')),
  });

  return <DataTable table={table} />;
};
