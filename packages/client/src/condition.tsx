import { ComponentProps } from 'react';
import { Control, FieldPathByValue, FieldValues } from 'react-hook-form';
import { Condition } from '@the-dev-tools/spec/condition/v1/condition_pb';
import { FieldLabel, FieldLabelProps } from '@the-dev-tools/ui/field';
import { ReferenceFieldRHF } from '~reference';

interface ConditionFieldProps<
  TFieldValues extends FieldValues,
  TPath extends FieldPathByValue<TFieldValues, Condition['$typeName']>,
> extends Omit<ComponentProps<'div'>, 'children'> {
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
  ...props
}: ConditionFieldProps<TFieldValues, TPath>) => {
  const resolvedControl = control as unknown as Control<{ condition: Condition }>;
  const resolvedPath = path as 'condition';

  return (
    <div {...props}>
      {label && <FieldLabel>{label}</FieldLabel>}

      <ReferenceFieldRHF
        allowFiles
        control={resolvedControl}
        name={`${resolvedPath}.comparison.expression`}
        placeholder='Enter value to compare'
        readOnly={isReadOnly ?? false}
      />
    </div>
  );
};
