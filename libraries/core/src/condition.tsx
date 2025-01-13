import { Control, Controller, FieldPathByValue, FieldValues } from 'react-hook-form';

import { ComparisonKind, Condition } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { ReferenceField } from './reference';

interface ConditionFieldProps<
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
> {
  control: Control<TFieldValues>;
  path: TPath extends `${infer Path}.$typeName` ? Path : never;
}

export const ConditionField = <
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
>({
  control,
  path,
}: ConditionFieldProps<TFieldValues, TPath>) => {
  const resolvedControl = control as unknown as Control<{ condition: Condition }>;
  const resolvedPath = path as 'condition';

  return (
    <div className={tw`flex items-center gap-2`}>
      <Controller
        control={resolvedControl}
        name={`${resolvedPath}.comparison.path`}
        defaultValue={[]}
        render={({ field }) => (
          <ReferenceField path={field.value} onSelect={field.onChange} buttonClassName={tw`flex-[2]`} />
        )}
      />

      <SelectRHF
        control={resolvedControl}
        name={`${resolvedPath}.comparison.kind`}
        className={tw`h-full flex-1`}
        triggerClassName={tw`h-full justify-between`}
        aria-label='Comparison Method'
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
      />
    </div>
  );
};
