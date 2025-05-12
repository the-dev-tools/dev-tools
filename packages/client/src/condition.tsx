import { ComponentProps } from 'react';
import { Control, FieldPathByValue, FieldValues } from 'react-hook-form';

import { Condition } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { FieldLabel, FieldLabelProps } from '@the-dev-tools/ui/field';
import { MixinProps, splitProps } from '@the-dev-tools/ui/mixin-props';
import { ReferenceFieldRHF } from '~reference';

interface ConditionFieldProps<
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
> extends MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
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
  isReadOnly,
  label,
  path,
  ...mixProps
}: ConditionFieldProps<TFieldValues, TPath>) => {
  const props = splitProps(mixProps, 'label');

  const resolvedControl = control as unknown as Control<{ condition: Condition }>;
  const resolvedPath = path as 'condition';

  return (
    <div {...props.rest}>
      {label && <FieldLabel {...props.label}>{label}</FieldLabel>}

      <ReferenceFieldRHF
        control={resolvedControl}
        name={`${resolvedPath}.comparison.expression`}
        placeholder='Enter value to compare'
        readOnly={isReadOnly ?? false}
      />
    </div>
  );
};
