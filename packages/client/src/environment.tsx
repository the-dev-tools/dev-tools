import { getRouteApi, useRouteContext } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { Suspense } from 'react';
import {
  Collection,
  Dialog,
  DialogTrigger,
  MenuTrigger,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  Tooltip,
  TooltipTrigger,
} from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';

import { EnvironmentListItem } from '@the-dev-tools/spec/environment/v1/environment_pb';
import {
  EnvironmentCreateEndpoint,
  EnvironmentDeleteEndpoint,
  EnvironmentListEndpoint,
  EnvironmentUpdateEndpoint,
} from '@the-dev-tools/spec/meta/environment/v1/environment.endpoints.ts';
import {
  VariableCreateEndpoint,
  VariableDeleteEndpoint,
  VariableListEndpoint,
  VariableUpdateEndpoint,
} from '@the-dev-tools/spec/meta/variable/v1/variable.endpoints.ts';
import { VariableListItemEntity } from '@the-dev-tools/spec/meta/variable/v1/variable.entities.ts';
import {
  WorkspaceGetEndpoint,
  WorkspaceUpdateEndpoint,
} from '@the-dev-tools/spec/meta/workspace/v1/workspace.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { GlobalEnvironmentIcon, Spinner, VariableIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useMutate, useQuery } from '~data-client';

import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
} from './form-table';
import { ImportDialog } from './workspace/import';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const EnvironmentsWidget = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { workspaceId } = workspaceRoute.useLoaderData();

  // TODO: fetch in parallel
  const { selectedEnvironmentId } = useQuery(WorkspaceGetEndpoint, { workspaceId });
  const { items: environments } = useQuery(EnvironmentListEndpoint, { workspaceId });

  const selectedEnvironmentIdCan = Ulid.construct(selectedEnvironmentId).toCanonical();

  return (
    <div className={tw`flex gap-1 border-b border-slate-200 p-3`}>
      <Select
        aria-label='Environment'
        listBoxItems={environments}
        onSelectionChange={async (selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          await dataClient.fetch(WorkspaceUpdateEndpoint, { selectedEnvironmentId, workspaceId });
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
        <TooltipTrigger delay={750}>
          <Button className={tw`p-1`} variant='ghost'>
            <GlobalEnvironmentIcon className={tw`size-4 text-slate-500`} />
          </Button>
          <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
            Manage Variables & Environments
          </Tooltip>
        </TooltipTrigger>

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

                    <TooltipTrigger delay={750}>
                      <Button
                        className={tw`bg-slate-200 p-0.5`}
                        onPress={() =>
                          dataClient.fetch(EnvironmentCreateEndpoint, {
                            name: 'New Environment',
                            workspaceId,
                          })
                        }
                        variant='ghost'
                      >
                        <FiPlus className={tw`size-4 text-slate-500`} />
                      </Button>
                      <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
                        Add New Environment
                      </Tooltip>
                    </TooltipTrigger>
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
  const { dataClient } = useRouteContext({ from: '__root__' });

  const [environmentUpdate, environmentUpdateLoading] = useMutate(EnvironmentUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => environmentUpdate({ environmentId, name: _ }),
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
            aria-label='Environment name'
            inputClassName={tw`-my-1 py-1 font-semibold leading-none tracking-tight text-slate-800`}
            isDisabled={environmentUpdateLoading}
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

              <MenuItem
                onAction={() => dataClient.fetch(EnvironmentDeleteEndpoint, { environmentId })}
                variant='danger'
              >
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
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { items } = useQuery(VariableListEndpoint, { environmentId });

  const table = useReactTable({
    columns: [
      columnCheckboxField<VariableListItemEntity>('enabled', { meta: { divider: false } }),
      columnReferenceField<VariableListItemEntity>('name'),
      columnReferenceField<VariableListItemEntity>('value', { allowFiles: true }),
      columnTextField<VariableListItemEntity>('description', { meta: { divider: false } }),
      columnActionsCommon<VariableListItemEntity>({
        onDelete: (_) => dataClient.fetch(VariableDeleteEndpoint, { variableId: _.variableId }),
      }),
    ],
    data: items,
  });

  const formTable = useFormTable({
    createLabel: 'New variable',
    items,
    onCreate: () =>
      dataClient.fetch(VariableCreateEndpoint, {
        enabled: true,
        environmentId,
        name: `VARIABLE_${items.length}`,
      }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(VariableUpdateEndpoint, item),
    primaryColumn: 'name',
  });

  return <DataTable {...formTable} table={table} />;
};
