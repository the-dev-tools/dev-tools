import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { Array, flow, HashMap, MutableHashMap, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { useMemo, useState } from 'react';
import { Dialog, DialogTrigger } from 'react-aria-components';
import { FieldPath, FieldValues, useController } from 'react-hook-form';
import { LuLink } from 'react-icons/lu';

import { EnvironmentListItem } from '@the-dev-tools/spec/environment/v1/environment_pb';
import { environmentList } from '@the-dev-tools/spec/environment/v1/environment-EnvironmentService_connectquery';
import { variableList } from '@the-dev-tools/spec/variable/v1/variable-VariableService_connectquery';
import { Workspace } from '@the-dev-tools/spec/workspace/v1/workspace_pb';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem, DropdownListBox, DropdownPopover } from '@the-dev-tools/ui/dropdown';
import { controllerPropKeys } from '@the-dev-tools/ui/react-hook-form';
import { TextField, TextFieldRHF, TextFieldRHFProps } from '@the-dev-tools/ui/text-field';

interface TextFieldWithVariablesProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends TextFieldRHFProps<TFieldValues, TName> {
  workspaceId: Workspace['workspaceId'];
}

export const TextFieldWithVariables = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  workspaceId,
  ...props
}: TextFieldWithVariablesProps<TFieldValues, TName>) => {
  const variableQuery = useConnectQuery(variableList, { workspaceId });
  const environmentQuery = useConnectQuery(environmentList, { workspaceId });

  const [filter, setFilter] = useState('');

  const variableEnvironments = useMemo(() => {
    if (!variableQuery.isSuccess || !environmentQuery.isSuccess) return [];
    const environmentMap = pipe(
      environmentQuery.data.items,
      Array.map((item) => [Ulid.construct(item.environmentId).toRaw(), item] as const),
      HashMap.fromIterable,
    );
    const variableEnvironmentMap = MutableHashMap.empty<string, EnvironmentListItem[]>();
    variableQuery.data.items.forEach((variable) => {
      const environment = HashMap.get(environmentMap, Ulid.construct(variable.environmentId).toRaw());
      if (Option.isNone(environment)) return;
      MutableHashMap.modifyAt(
        variableEnvironmentMap,
        variable.name,
        flow(
          Option.getOrElse((): EnvironmentListItem[] => []),
          Array.append(environment.value),
          Option.some,
        ),
      );
    });
    return Array.fromIterable(variableEnvironmentMap);
  }, [environmentQuery.data?.items, environmentQuery.isSuccess, variableQuery.data?.items, variableQuery.isSuccess]);

  const controllerProps = Struct.pick(props, ...controllerPropKeys);
  const { field } = useController(controllerProps);

  return (
    <div className='flex'>
      <TextFieldRHF variant='table-cell' className='flex-1' {...props} />

      <DialogTrigger>
        <Button variant='ghost'>
          <LuLink />
        </Button>

        <DropdownPopover className='max-w-80'>
          <Dialog className='outline-none'>
            {({ close }) => (
              <>
                {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
                <TextField aria-label='Search variables' value={filter} onChange={setFilter} autoFocus />

                <DropdownListBox
                  aria-label='Variables'
                  items={variableEnvironments.filter(([name]) => name.toLowerCase().includes(filter.toLowerCase()))}
                  selectionMode='none'
                >
                  {([name, environments]) => {
                    const value = `{{ ${name} }}`;
                    return (
                      <DropdownItem
                        id={name}
                        textValue={name}
                        className='flex items-center gap-4 p-1 text-xs'
                        onAction={() => {
                          field.onChange(value);
                          close();
                          setFilter('');
                        }}
                      >
                        <div className='text-nowrap'>{value}</div>
                        <div className='flex flex-1 flex-wrap items-center justify-end gap-2'>
                          {environments.map((item) => {
                            const environmentIdCan = Ulid.construct(item.environmentId).toCanonical();
                            return (
                              <div key={environmentIdCan} className='rounded bg-neutral-300 px-1.5 py-0.5'>
                                {item.isGlobal ? 'Global' : item.name}
                              </div>
                            );
                          })}
                        </div>
                      </DropdownItem>
                    );
                  }}
                </DropdownListBox>
              </>
            )}
          </Dialog>
        </DropdownPopover>
      </DialogTrigger>
    </div>
  );
};
