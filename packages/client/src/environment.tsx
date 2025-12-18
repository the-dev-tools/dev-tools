'use no memo'; // TODO: fix variable table incorrect first render with compiler

import { eq, useLiveQuery } from '@tanstack/react-db';
import { Array, Option, pipe, Predicate } from 'effect';
import { Ulid } from 'id128';
import { Suspense, useMemo, useState } from 'react';
import {
  Button as AriaButton,
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  Dialog,
  DialogTrigger,
  Key,
  MenuTrigger,
  ToggleButton,
  Tooltip,
  TooltipTrigger,
  useDragAndDrop,
} from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import {
  EnvironmentInsertSchema,
  EnvironmentVariable,
} from '@the-dev-tools/spec/buf/api/environment/v1/environment_pb';
import {
  EnvironmentCollectionSchema,
  EnvironmentVariableCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/environment';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { GlobalEnvironmentIcon, VariableIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Protobuf, useApiCollection } from '~/api';
import { workspaceRouteApi } from '~/routes';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';
import { ExportDialog } from '~/workspace/export';
import { ImportDialogTrigger } from '~/workspace/import';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
  useFormTableAddRow,
} from './form-table';

export const EnvironmentsWidget = () => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const selectedEnvironmentIdCan = pipe(
    useLiveQuery(
      (_) =>
        _.from({ workspace: workspaceCollection })
          .where((_) => eq(_.workspace.workspaceId, workspaceId))
          .select((_) => pick(_.workspace, 'selectedEnvironmentId'))
          .findOne(),
      [workspaceCollection, workspaceId],
    ),
    (_) => Option.fromNullable(_.data?.selectedEnvironmentId),
    Option.map((_) => Ulid.construct(_).toCanonical()),
    Option.getOrNull,
  );

  const environmentCollection = useApiCollection(EnvironmentCollectionSchema);

  const { data: environments } = useLiveQuery(
    (_) =>
      _.from({ environment: environmentCollection })
        .where((_) => eq(_.environment.workspaceId, workspaceId))
        .orderBy((_) => _.environment.order)
        .select((_) => pick(_.environment, 'environmentId', 'name', 'isGlobal', 'order')),
    [environmentCollection, workspaceId],
  );

  return (
    <div className={tw`flex gap-1 border-b border-slate-200 p-3`}>
      <Select
        aria-label='Environment'
        items={environments}
        onSelectionChange={(selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          workspaceCollection.utils.update({ selectedEnvironmentId, workspaceId });
        }}
        selectedKey={selectedEnvironmentIdCan}
        triggerClassName={tw`justify-start p-0`}
        triggerVariant='ghost'
      >
        {(item) => {
          const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
          return (
            <SelectItem id={environmentIdCan} textValue={item.name}>
              <div className={tw`flex items-center gap-2`}>
                <div
                  className={tw`flex size-6 items-center justify-center rounded-md bg-slate-200 text-xs text-slate-500`}
                >
                  {item.isGlobal ? <VariableIcon /> : item.name[0]}
                </div>
                <span className={tw`text-md leading-5 font-semibold tracking-tight text-slate-800`}>
                  {item.isGlobal ? 'Global Environment' : item.name}
                </span>
              </div>
            </SelectItem>
          );
        }}
      </Select>

      <div className={tw`flex-1`} />

      <ImportDialogTrigger />

      <ExportDialog />

      <DialogTrigger>
        <TooltipTrigger delay={750}>
          <Button className={tw`p-1`} variant='ghost'>
            <GlobalEnvironmentIcon className={tw`size-4 text-slate-500`} />
          </Button>
          <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
            Manage Variables & Environments
          </Tooltip>
        </TooltipTrigger>
        <EnvironmentModal />
      </DialogTrigger>
    </div>
  );
};

const EnvironmentModal = () => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const environmentCollection = useApiCollection(EnvironmentCollectionSchema);

  const { data: environments } = useLiveQuery(
    (_) =>
      _.from({ environment: environmentCollection })
        .where((_) => eq(_.environment.workspaceId, workspaceId))
        .orderBy((_) => _.environment.order)
        .select((_) => pick(_.environment, 'environmentId', 'name', 'order')),
    [environmentCollection, workspaceId],
  );

  const globalKey = pipe(
    useLiveQuery(
      (_) =>
        _.from({ environment: environmentCollection })
          .where((_) => eq(_.environment.workspaceId, workspaceId))
          .where((_) => eq(_.environment.isGlobal, true))
          .select((_) => pick(_.environment, 'environmentId'))
          .findOne(),
      [environmentCollection, workspaceId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.map((_) => environmentCollection.utils.getKey(_)),
    Option.getOrUndefined,
  );

  const [selectedKey, setSelectedKey] = useState<Key | undefined>(globalKey);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(environmentCollection),
    renderDropIndicator: () => <DropIndicatorHorizontal />,
  });

  return (
    <Modal>
      <Dialog className={tw`h-full outline-hidden`}>
        {({ close }) => (
          <div className={tw`flex h-full`}>
            <div className={tw`flex w-64 flex-col border-r border-slate-200 bg-slate-50 p-4 tracking-tight`}>
              <div className={tw`mb-4`}>
                <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-slate-800`}>Variable Settings</div>
                <div className={tw`text-xs leading-4 text-slate-500`}>Manage variables & environment</div>
              </div>

              {globalKey && (
                <ToggleButton
                  className={({ isSelected }) =>
                    twJoin(
                      tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                      isSelected && tw`bg-slate-200`,
                    )
                  }
                  isSelected={selectedKey === globalKey}
                  onChange={(isSelected) => {
                    if (isSelected && globalKey) setSelectedKey(globalKey);
                  }}
                >
                  <VariableIcon className={tw`size-4 text-slate-500`} />
                  <span className={tw`text-md leading-5 font-semibold`}>Global Variables</span>
                </ToggleButton>
              )}

              <div className={tw`mt-3 mb-1 flex items-center justify-between py-0.5`}>
                <span className={tw`text-md leading-5 text-slate-400`}>Environments</span>

                <TooltipTrigger delay={750}>
                  <Button
                    className={tw`bg-slate-200 p-0.5`}
                    onPress={async () => {
                      const environment = Protobuf.create(EnvironmentInsertSchema, {
                        environmentId: Ulid.generate().bytes,
                        name: 'New Environment',
                        order: await getNextOrder(environmentCollection),
                        workspaceId,
                      });

                      environmentCollection.utils.insert(environment);

                      setSelectedKey(environmentCollection.utils.getKey(environment));
                    }}
                    variant='ghost'
                  >
                    <FiPlus className={tw`size-4 text-slate-500`} />
                  </Button>
                  <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
                    Add New Environment
                  </Tooltip>
                </TooltipTrigger>
              </div>

              <AriaListBox
                aria-label='Environments'
                dependencies={[{}]}
                dragAndDropHooks={dragAndDropHooks}
                items={environments.filter((_) => environmentCollection.utils.getKey(_) !== globalKey)}
                onSelectionChange={(keys) => {
                  if (!Predicate.isSet(keys) || keys.size !== 1) return;
                  const [key] = keys.values();
                  setSelectedKey(key);
                }}
                selectedKeys={Array.fromNullable(selectedKey)}
                selectionMode='single'
              >
                {(_) => (
                  <AriaListBoxItem
                    className={({ isSelected }) =>
                      twJoin(
                        tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                        isSelected && tw`bg-slate-200`,
                      )
                    }
                    id={environmentCollection.utils.getKey(_)}
                    textValue={_.name}
                  >
                    <div
                      className={tw`
                        flex size-4 items-center justify-center rounded-sm bg-slate-300 text-xs leading-3 text-slate-500
                      `}
                    >
                      {_.name[0]}
                    </div>
                    <span className={tw`text-md leading-5 font-semibold`}>{_.name}</span>
                  </AriaListBoxItem>
                )}
              </AriaListBox>
            </div>

            <div className={tw`flex h-full min-w-0 flex-1 flex-col`}>
              {selectedKey && <EnvironmentPanel id={selectedKey.toString()} />}
              <div className={tw`flex-1`} />
              <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
                <Button onPress={close} variant='primary'>
                  Close
                </Button>
              </div>
            </div>
          </div>
        )}
      </Dialog>
    </Modal>
  );
};

interface EnvironmentPanelProps {
  id: string;
}

const EnvironmentPanel = ({ id }: EnvironmentPanelProps) => {
  const environmentCollection = useApiCollection(EnvironmentCollectionSchema);

  const { environmentId } = useMemo(
    () => environmentCollection.utils.parseKeyUnsafe(id),
    [environmentCollection.utils, id],
  );

  const { data } = useLiveQuery(
    (_) =>
      _.from({ environment: environmentCollection })
        .where((_) => eq(_.environment.environmentId, environmentId))
        .select((_) => pick(_.environment, 'name', 'isGlobal'))
        .findOne(),
    [environmentCollection, environmentId],
  );

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => environmentCollection.utils.update({ environmentId, name: _ }),
    value: data?.name ?? '',
  });

  if (!data) return null;

  const { isGlobal, name } = data;

  return (
    <div className={tw`h-full px-6 py-4`}>
      <div className={tw`mb-4 flex items-center gap-2`} onContextMenu={onContextMenu}>
        {isGlobal ? (
          <VariableIcon className={tw`size-6 text-slate-500`} />
        ) : (
          <div
            className={tw`
              flex size-6 items-center justify-center rounded-md bg-slate-300 text-xs leading-3 text-slate-500
            `}
          >
            {name[0]}
          </div>
        )}

        {isEditing ? (
          <TextInputField
            aria-label='Environment name'
            inputClassName={tw`-my-1 py-1 leading-none font-semibold tracking-tight text-slate-800`}
            {...textFieldProps}
          />
        ) : (
          <AriaButton
            className={tw`max-w-full cursor-text truncate leading-5 font-semibold tracking-tight text-slate-800`}
            isDisabled={isGlobal}
            onContextMenu={onContextMenu}
            onPress={() => void edit()}
          >
            {isGlobal ? 'Global Variables' : name}
          </AriaButton>
        )}

        <div className={tw`flex-1`} />

        {!isGlobal && (
          <MenuTrigger {...menuTriggerProps}>
            <Button className={tw`p-1`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem onAction={() => environmentCollection.utils.delete({ environmentId })} variant='danger'>
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='lg' />
          </div>
        }
      >
        <VariablesTable environmentId={environmentId} />
      </Suspense>
    </div>
  );
};

interface VariablesTableProps {
  environmentId: Uint8Array;
}

export const VariablesTable = ({ environmentId }: VariablesTableProps) => {
  const variableColleciton = useApiCollection(EnvironmentVariableCollectionSchema);

  const { data: variables } = useLiveQuery(
    (_) =>
      _.from({ variable: variableColleciton })
        .where((_) => eq(_.variable.environmentId, environmentId))
        .orderBy((_) => _.variable.order),
    [environmentId, variableColleciton],
  );

  const table = useReactTable({
    columns: [
      columnCheckboxField<EnvironmentVariable>('enabled', { meta: { divider: false } }),
      columnReferenceField<EnvironmentVariable>('key', { meta: { isRowHeader: true } }),
      columnReferenceField<EnvironmentVariable>('value', { allowFiles: true }),
      columnTextField<EnvironmentVariable>('description', { meta: { divider: false } }),
      columnActionsCommon<EnvironmentVariable>({
        onDelete: (_) => variableColleciton.utils.delete(pick(_, 'environmentVariableId')),
      }),
    ],
    data: variables,
    getRowId: (_) => variableColleciton.utils.getKey(_),
  });

  const formTable = useFormTable<EnvironmentVariable>({
    onUpdate: ({ $typeName: _, ...item }) => variableColleciton.utils.update(item),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New variable',
    items: variables,
    onCreate: async () =>
      variableColleciton.utils.insert({
        enabled: true,
        environmentId,
        environmentVariableId: Ulid.generate().bytes,
        key: `VARIABLE_${variables.length}`,
        order: await getNextOrder(variableColleciton),
      }),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(variableColleciton),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <DataTable
      {...formTable}
      {...addRow}
      aria-label='Environment variables'
      dragAndDropHooks={dragAndDropHooks}
      table={table}
    />
  );
};
