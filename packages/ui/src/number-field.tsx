import { Struct } from 'effect';
import { RefAttributes } from 'react';
import { mergeProps } from 'react-aria';
import * as RAC from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { FiMinus, FiPlus } from 'react-icons/fi';
import { FieldLabel, FieldLabelProps } from './field';
import { focusVisibleRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

// Number field

export interface NumberFieldProps extends RAC.NumberFieldProps, RefAttributes<HTMLDivElement> {
  groupClassName?: RAC.GroupProps['className'];
  label?: FieldLabelProps['children'];
}

export const NumberField = ({ className = '', groupClassName, label, ...props }: NumberFieldProps) => (
  <RAC.NumberField className={className} {...props}>
    {label && <FieldLabel>{label}</FieldLabel>}

    <RAC.Group
      className={composeTailwindRenderProps(
        groupClassName,
        focusVisibleRingStyles(),
        tw`flex min-w-0 rounded-md border border-slate-200 text-md leading-5 text-slate-800`,
      )}
    >
      <RAC.Button className={tw`flex size-8 items-center justify-center border-r border-slate-200`} slot='decrement'>
        <FiMinus />
      </RAC.Button>

      <RAC.Input className={tw`min-w-0 flex-1 px-3 outline-hidden`} />

      <RAC.Button className={tw`flex size-8 items-center justify-center border-l border-slate-200`} slot='increment'>
        <FiPlus />
      </RAC.Button>
    </RAC.Group>
  </RAC.NumberField>
);

// Number field RHF wrapper

export interface NumberFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>
  extends Omit<NumberFieldProps, ControllerPropKeys>, UseControllerProps<TFieldValues, TName> {}

export const NumberFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: NumberFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const {
    field: { ref, ...field },
    fieldState,
  } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: NumberFieldProps = {
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
    value: field.value,
  };

  return <NumberField {...mergeProps(fieldProps, forwardedProps)} ref={ref} />;
};
