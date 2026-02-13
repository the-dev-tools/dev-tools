import * as Protobuf from '@bufbuild/protobuf';
import { eq, Query, useLiveQuery } from '@tanstack/react-db';
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
import { EnvironmentInsertSchema } from '@the-dev-tools/spec/buf/api/environment/v1/environment_pb';
import {
  EnvironmentCollectionSchema,
  EnvironmentVariableCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/environment';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { GlobalEnvironmentIcon, VariableIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { ReferenceField } from '~/features/expression';
import { ColumnActionDelete } from '~/features/form-table';
import { useApiCollection } from '~/shared/api';
import { eqStruct, getNextOrder, handleCollectionReorder, LiveQuery, pick, pickStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { ExportDialog } from '~/widgets/export';
import { ImportDialogTrigger } from '~/widgets/import';

export const EnvironmentsWidget = () => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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
    <div className={tw`flex gap-1 border-b border-neutral p-3`}>
      <Select
        aria-label='Environment'
        items={environments}
        onChange={(selectedEnvironmentIdCan) => {
          const selectedEnvironmentId = Ulid.fromCanonical(selectedEnvironmentIdCan as string).bytes;
          workspaceCollection.utils.update({ selectedEnvironmentId, workspaceId });
        }}
        triggerClassName={tw`justify-start p-0`}
        triggerVariant='ghost'
        value={selectedEnvironmentIdCan}
      >
        {(item) => {
          const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
          return (
            <SelectItem id={environmentIdCan} textValue={item.name}>
              <div className={tw`flex items-center gap-2`}>
                <div
                  className={tw`
                    flex size-6 items-center justify-center rounded-md bg-neutral text-xs text-on-neutral-low
                  `}
                >
                  {item.isGlobal ? <VariableIcon /> : item.name[0]}
                </div>
                <span className={tw`text-md leading-5 font-semibold tracking-tight text-on-neutral`}>
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
            <GlobalEnvironmentIcon className={tw`size-4 text-on-neutral-low`} />
          </Button>
          <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
            Manage Variables & Environments
          </Tooltip>
        </TooltipTrigger>
        <EnvironmentModal />
      </DialogTrigger>
    </div>
  );
};

const EnvironmentModal = () => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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
            <div className={tw`flex w-64 flex-col border-r border-neutral bg-neutral-lower p-4 tracking-tight`}>
              <div className={tw`mb-4`}>
                <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-on-neutral`}>Variable Settings</div>
                <div className={tw`text-xs leading-4 text-on-neutral-low`}>Manage variables & environment</div>
              </div>

              {globalKey && (
                <ToggleButton
                  className={({ isSelected }) =>
                    twJoin(
                      tw`-mx-2 flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-sm`,
                      isSelected && tw`bg-neutral`,
                    )
                  }
                  isSelected={selectedKey === globalKey}
                  onChange={(isSelected) => {
                    if (isSelected && globalKey) setSelectedKey(globalKey);
                  }}
                >
                  <VariableIcon className={tw`size-4 text-on-neutral-low`} />
                  <span className={tw`text-md leading-5 font-semibold`}>Global Variables</span>
                </ToggleButton>
              )}

              <div className={tw`mt-3 mb-1 flex items-center justify-between py-0.5`}>
                <span className={tw`text-md leading-5 text-neutral-higher`}>Environments</span>

                <TooltipTrigger delay={750}>
                  <Button
                    className={tw`bg-neutral p-0.5`}
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
                    <FiPlus className={tw`size-4 text-on-neutral-low`} />
                  </Button>
                  <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
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
                        isSelected && tw`bg-neutral`,
                      )
                    }
                    id={environmentCollection.utils.getKey(_)}
                    textValue={_.name}
                  >
                    <div
                      className={tw`
                        flex size-4 items-center justify-center rounded-sm bg-neutral-high text-xs leading-3
                        text-on-neutral-low
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
              <div className={tw`flex justify-end gap-2 border-t border-neutral px-6 py-3`}>
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
          <VariableIcon className={tw`size-6 text-on-neutral-low`} />
        ) : (
          <div
            className={tw`
              flex size-6 items-center justify-center rounded-md bg-neutral-high text-xs leading-3 text-on-neutral-low
            `}
          >
            {name[0]}
          </div>
        )}

        {isEditing ? (
          <TextInputField
            aria-label='Environment name'
            inputClassName={tw`-my-1 py-1 leading-none font-semibold tracking-tight text-on-neutral`}
            {...textFieldProps}
          />
        ) : (
          <AriaButton
            className={tw`max-w-full cursor-text truncate leading-5 font-semibold tracking-tight text-on-neutral`}
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
              <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
  const collection = useApiCollection(EnvironmentVariableCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where(eqStruct({ environmentId }))
        .select(pickStruct('environmentVariableId', 'order'))
        .orderBy((_) => _.item.order),
    [environmentId, collection],
  );

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table aria-label='Environment variables' dragAndDropHooks={dragAndDropHooks}>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        <TableColumn>Description</TableColumn>
        <TableColumn width={32} />
      </TableHeader>

      <TableBody items={items}>
        {({ environmentVariableId }) => {
          const query = new Query().from({ item: collection }).where(eqStruct({ environmentVariableId })).findOne();

          return (
            <TableRow id={collection.utils.getKey({ environmentVariableId })}>
              <TableCell className={tw`border-r-0`}>
                <LiveQuery query={() => query.select(pickStruct('enabled'))}>
                  {({ data }) => (
                    <Checkbox
                      aria-label='Enabled'
                      isSelected={data?.enabled ?? false}
                      isTableCell
                      onChange={(_) => void collection.utils.update({ enabled: _, environmentVariableId })}
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('key'))}>
                  {({ data }) => (
                    <ReferenceField
                      className='flex-1'
                      kind='StringExpression'
                      onChange={(_) => void collection.utils.update({ environmentVariableId, key: _ })}
                      placeholder={`Enter key`}
                      value={data?.key ?? ''}
                      variant='table-cell'
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('value'))}>
                  {({ data }) => (
                    <ReferenceField
                      allowFiles
                      className='flex-1'
                      kind='StringExpression'
                      onChange={(_) => void collection.utils.update({ environmentVariableId, value: _ })}
                      placeholder={`Enter value`}
                      value={data?.value ?? ''}
                      variant='table-cell'
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('description'))}>
                  {({ data }) => (
                    <TextInputField
                      aria-label='Description'
                      className='flex-1'
                      isTableCell
                      onChange={(_) => void collection.utils.update({ description: _, environmentVariableId })}
                      placeholder={`Enter description`}
                      value={data?.description ?? ''}
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDelete onDelete={() => void collection.utils.delete({ environmentVariableId })} />
              </TableCell>
            </TableRow>
          );
        }}
      </TableBody>

      <TableFooter>
        <Button
          className={tw`w-full justify-start -outline-offset-4`}
          onPress={async () => {
            collection.utils.insert({
              enabled: true,
              environmentId,
              environmentVariableId: Ulid.generate().bytes,
              key: `VARIABLE_${items.length}`,
              order: await getNextOrder(collection),
            });
          }}
          variant='ghost'
        >
          <FiPlus className={tw`size-4 text-on-neutral-low`} />
          New variable
        </Button>
      </TableFooter>
    </Table>
  );
};
