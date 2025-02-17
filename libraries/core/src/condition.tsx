import { ComponentProps } from 'react';
import { Control, Controller, FieldPathByValue, FieldValues } from 'react-hook-form';
import { twMerge } from 'tailwind-merge';

import { ComparisonKind, Condition } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { FieldLabel, FieldLabelProps } from '@the-dev-tools/ui/field';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { ReferenceField } from './reference';

interface ConditionFieldProps<
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
> extends Omit<ComponentProps<'div'>, 'children'>,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'group', Omit<ComponentProps<'div'>, 'children'>> {
  control: Control<TFieldValues>;
  path: TPath extends `${infer Path}.$typeName` ? Path : never;
  label?: FieldLabelProps['children'];
  isReadOnly?: boolean | undefined;
}

export const ConditionField = <
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
>({
  control,
  path,
  label,
  groupClassName,
  isReadOnly,
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
          name={`${resolvedPath}.comparison.path`}
          defaultValue={[]}
          render={({ field }) => (
            <ReferenceField
              path={field.value}
              onSelect={field.onChange}
              buttonClassName={tw`flex-[2]`}
              isReadOnly={isReadOnly}
            />
          )}
        />

        <SelectRHF
          control={resolvedControl}
          name={`${resolvedPath}.comparison.kind`}
          className={tw`h-full flex-1`}
          triggerClassName={tw`h-full justify-between`}
          aria-label='Comparison Method'
          isDisabled={isReadOnly ?? false}
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
          control={resolvedControl}
          name={`${resolvedPath}.comparison.value`}
          className={tw`h-full flex-[2]`}
          inputClassName={tw`h-full`}
          inputPlaceholder='Enter comparison value'
          isReadOnly={isReadOnly ?? false}
        />
      </div>
    </div>
  );
};
