import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { Struct } from 'effect';
import { useState } from 'react';
import { Dialog, DialogTrigger } from 'react-aria-components';
import { FieldPath, FieldValues, useController } from 'react-hook-form';
import { LuLink } from 'react-icons/lu';

import { EnvironmentType } from '@the-dev-tools/protobuf/environment/v1/environment_pb';
import { getAllVariables } from '@the-dev-tools/protobuf/environment/v1/environment-EnvironmentService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { DropdownItem, DropdownListBox, DropdownPopover } from '@the-dev-tools/ui/dropdown';
import { controllerPropKeys } from '@the-dev-tools/ui/react-hook-form';
import { TextField, TextFieldRHF, TextFieldRHFProps } from '@the-dev-tools/ui/text-field';

interface TextFieldWithVariablesProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends TextFieldRHFProps<TFieldValues, TName> {
  workspaceId: string;
}

export const TextFieldWithVariables = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  workspaceId,
  ...props
}: TextFieldWithVariablesProps<TFieldValues, TName>) => {
  const variableQuery = useConnectQuery(getAllVariables, { workspaceId });
  const [filter, setFilter] = useState('');

  const controllerProps = Struct.pick(props, ...controllerPropKeys);
  const { field } = useController(controllerProps);

  return (
    <div className='flex'>
      <TextFieldRHF variant='table-cell' className='flex-1' {...props} />

      <DialogTrigger>
        <Button kind='placeholder' variant='placeholder ghost'>
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
                  items={
                    variableQuery.data?.items.filter((item) =>
                      item.variableKey.toLowerCase().includes(filter.toLowerCase()),
                    ) ?? []
                  }
                  selectionMode='none'
                >
                  {(item) => {
                    const value = `{{ ${item.variableKey} }}`;
                    return (
                      <DropdownItem
                        id={item.variableKey}
                        textValue={item.variableKey}
                        className='flex items-center gap-4 p-1 text-xs'
                        onAction={() => {
                          field.onChange(value);
                          close();
                          setFilter('');
                        }}
                      >
                        <div className='text-nowrap'>{value}</div>
                        <div className='flex flex-1 flex-wrap items-center justify-end gap-2'>
                          {item.environment.map((item) => (
                            <div key={item.id} className='rounded bg-neutral-300 px-1.5 py-0.5'>
                              {item.type === EnvironmentType.GLOBAL ? 'Global' : item.name}
                            </div>
                          ))}
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
