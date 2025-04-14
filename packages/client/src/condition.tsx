import { ComponentProps } from 'react';
import { Control, Controller, FieldPathByValue, FieldValues } from 'react-hook-form';
import { twMerge } from 'tailwind-merge';

import { ComparisonKind, Condition } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { FieldLabel, FieldLabelProps } from '@the-dev-tools/ui/field';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { MixinProps, splitProps } from '@the-dev-tools/ui/mixin-props';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { ReferenceField } from './reference';

interface ConditionFieldProps<
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
> extends MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'group', Omit<ComponentProps<'div'>, 'children'>>,
    Omit<ComponentProps<'div'>, 'children'> {
  control: Control<TFieldValues>;
  isReadOnly?: boolean | undefined;
  label?: FieldLabelProps['children'];
  path: TPath extends `${infer Path}.$typeName` ? Path : never;
}

export const ConditionField = <
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
>({
  control,
  groupClassName,
  isReadOnly,
  label,
  path,
  ...mixProps
}: ConditionFieldProps<TFieldValues, TPath>) => {
  const props = splitProps(mixProps, 'label', 'group');

  const resolvedControl = control as unknown as Control<{ condition: Condition }>;
  const resolvedPath = path as 'condition';

  return (
    <div {...props.rest}>
      {label && <FieldLabel {...props.label}>{label}</FieldLabel>}

      <div className={twMerge(tw`flex items-center gap-2`, groupClassName)}>
        <Controller
          control={resolvedControl}
          defaultValue={[]}
          name={`${resolvedPath}.comparison.path`}
          render={({ field }) => (
            <Controller
              control={resolvedControl}
              name={`${resolvedPath}.comparison.value`}
              render={({ field: valueField }) => (
                <ReferenceField
                  buttonClassName={tw`flex-[2]`}
                  isReadOnly={isReadOnly}
                  onSelect={(keys, value) => {
                    field.onChange(keys);
                    // eslint-disable-next-line @typescript-eslint/no-base-to-string
                    if (value) valueField.onChange(String(value));
                  }}
                  path={field.value}
                />
              )}
            />
          )}
        />

        <SelectRHF
          aria-label='Comparison Method'
          className={tw`h-full flex-1`}
          control={resolvedControl}
          isDisabled={isReadOnly ?? false}
          name={`${resolvedPath}.comparison.kind`}
          triggerClassName={tw`h-8 justify-between`}
        >
          <ListBoxItem id={ComparisonKind.EQUAL}>is equal to</ListBoxItem>
          <ListBoxItem id={ComparisonKind.NOT_EQUAL}>is not equal to</ListBoxItem>
          <ListBoxItem id={ComparisonKind.CONTAINS}>contains</ListBoxItem>
          <ListBoxItem id={ComparisonKind.NOT_CONTAINS}>does not contain</ListBoxItem>
          <ListBoxItem id={ComparisonKind.GREATER}>is greater than</ListBoxItem>
          <ListBoxItem id={ComparisonKind.GREATER_OR_EQUAL}>is greater or equal to</ListBoxItem>
          <ListBoxItem id={ComparisonKind.LESS}>is less than</ListBoxItem>
          <ListBoxItem id={ComparisonKind.LESS_OR_EQUAL}>is less or equal to</ListBoxItem>
        </SelectRHF>

        <TextFieldRHF
          className={tw`h-8 flex-[2]`}
          control={resolvedControl}
          inputClassName={tw`h-full`}
          inputPlaceholder='Enter comparison value'
          isReadOnly={isReadOnly ?? false}
          name={`${resolvedPath}.comparison.value`}
        />
      </div>
    </div>
  );
};
