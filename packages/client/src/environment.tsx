import { Array, HashMap, Match, Option, pipe, Predicate } from 'effect';
import { Ulid } from 'id128';
import { Suspense, useState } from 'react';
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
import { EnvironmentListItem } from '@the-dev-tools/spec/environment/v1/environment_pb';
import {
  EnvironmentCreateEndpoint,
  EnvironmentDeleteEndpoint,
  EnvironmentListEndpoint,
  EnvironmentMoveEndpoint,
  EnvironmentUpdateEndpoint,
} from '@the-dev-tools/spec/meta/environment/v1/environment.endpoints.ts';
import {
  VariableCreateEndpoint,
  VariableDeleteEndpoint,
  VariableListEndpoint,
  VariableMoveEndpoint,
  VariableUpdateEndpoint,
} from '@the-dev-tools/spec/meta/variable/v1/variable.endpoints.ts';
import { VariableListItemEntity } from '@the-dev-tools/spec/meta/variable/v1/variable.entities.ts';
import {
  WorkspaceGetEndpoint,
  WorkspaceUpdateEndpoint,
} from '@the-dev-tools/spec/meta/workspace/v1/workspace.endpoints.ts';
import { MovePosition } from '@the-dev-tools/spec/resources/v1/resources_pb';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { GlobalEnvironmentIcon, VariableIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { Select } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useMutate, useQuery } from '~data-client';
import { rootRouteApi, workspaceRouteApi } from '~routes';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
} from './form-table';
import { ImportDialog } from './workspace/import';

export const EnvironmentsWidget = () => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

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
                <span className={tw`text-md leading-5 font-semibold tracking-tight text-slate-800`}>
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
        <EnvironmentModal />
      </DialogTrigger>
    </div>
  );
};

const EnvironmentModal = () => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const { items: environments } = useQuery(EnvironmentListEndpoint, { workspaceId });

  const environmentMap = pipe(
    Array.map(environments, (_) => [Ulid.construct(_.environmentId).toCanonical(), _] as const),
    HashMap.fromIterable,
  );

  const { global: [global] = [], rest = [] } = Array.groupBy(environments, (_) => (_.isGlobal ? 'global' : 'rest'));

  const globalIdCan = pipe(
    Option.fromNullable(global),
    Option.map((_) => Ulid.construct(_.environmentId).toCanonical()),
    Option.getOrUndefined,
  );

  const [selectedKey, setSelectedKey] = useState<Key | undefined>(globalIdCan);

  const environment = pipe(
    Option.liftPredicate(selectedKey, Predicate.isString),
    Option.flatMap((_) => HashMap.get(environmentMap, _)),
    Option.getOrNull,
  );

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(EnvironmentMoveEndpoint, {
          environmentId: Ulid.fromCanonical(sourceIdCan).bytes,
          position,
          targetEnvironmentId: Ulid.fromCanonical(targetIdCan).bytes,
          workspaceId,
        });
      }),
    renderDropIndicator: () => <div className={tw`relative z-10 h-0 w-full ring ring-violet-700`} />,
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

              <ToggleButton
                className={({ isSelected }) =>
                  twJoin(
                    tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                    isSelected && tw`bg-slate-200`,
                  )
                }
                isSelected={selectedKey === globalIdCan}
                onChange={(isSelected) => {
                  if (isSelected && globalIdCan) setSelectedKey(globalIdCan);
                }}
              >
                <VariableIcon className={tw`size-4 text-slate-500`} />
                <span className={tw`text-md leading-5 font-semibold`}>Global Variables</span>
              </ToggleButton>

              <div className={tw`mt-3 mb-1 flex items-center justify-between py-0.5`}>
                <span className={tw`text-md leading-5 text-slate-400`}>Environments</span>

                <TooltipTrigger delay={750}>
                  <Button
                    className={tw`bg-slate-200 p-0.5`}
                    onPress={async () => {
                      const { environmentId } = await dataClient.fetch(EnvironmentCreateEndpoint, {
                        name: 'New Environment',
                        workspaceId,
                      });

                      const environmentIdCan = Ulid.construct(environmentId).toCanonical();

                      setSelectedKey(environmentIdCan);
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
                dragAndDropHooks={dragAndDropHooks}
                items={rest}
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
                    id={Ulid.construct(_.environmentId).toCanonical()}
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
              {environment && <EnvironmentPanel environment={environment} />}
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
  environment: EnvironmentListItem;
}

const EnvironmentPanel = ({ environment: { environmentId, isGlobal, name } }: EnvironmentPanelProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const [environmentUpdate, environmentUpdateLoading] = useMutate(EnvironmentUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => environmentUpdate({ environmentId, name: _ }),
    value: name,
  });

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
          <TextField
            aria-label='Environment name'
            inputClassName={tw`-my-1 py-1 leading-none font-semibold tracking-tight text-slate-800`}
            isDisabled={environmentUpdateLoading}
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
  const { dataClient } = rootRouteApi.useRouteContext();

  const { items } = useQuery(VariableListEndpoint, { environmentId });

  const table = useReactTable({
    columns: [
      columnCheckboxField<VariableListItemEntity>('enabled', { meta: { divider: false } }),
      columnReferenceField<VariableListItemEntity>('name', { meta: { isRowHeader: true } }),
      columnReferenceField<VariableListItemEntity>('value', { allowFiles: true }),
      columnTextField<VariableListItemEntity>('description', { meta: { divider: false } }),
      columnActionsCommon<VariableListItemEntity>({
        onDelete: (_) => dataClient.fetch(VariableDeleteEndpoint, { variableId: _.variableId }),
      }),
    ],
    data: items,
    getRowId: (_) => Ulid.construct(_.variableId).toCanonical(),
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

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(VariableMoveEndpoint, {
          environmentId,
          position,
          targetVariableId: Ulid.fromCanonical(targetIdCan).bytes,
          variableId: Ulid.fromCanonical(sourceIdCan).bytes,
        });
      }),
    renderDropIndicator: () => <tr className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />,
  });

  return (
    <DataTable
      {...formTable}
      table={table}
      tableAria-label='Environment variables'
      tableDragAndDropHooks={dragAndDropHooks}
    />
  );
};
