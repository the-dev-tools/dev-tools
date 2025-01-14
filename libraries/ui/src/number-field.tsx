import { Struct } from 'effect';
import { Ref } from 'react';
import { mergeProps } from 'react-aria';
import {
  Button as AriaButton,
  ButtonProps as AriaButtonProps,
  Group as AriaGroup,
  GroupProps as AriaGroupProps,
  Input as AriaInput,
  InputProps as AriaInputProps,
  NumberField as AriaNumberField,
  NumberFieldProps as AriaNumberFieldProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { FiMinus, FiPlus } from 'react-icons/fi';
import { tv } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { FieldLabel, FieldLabelProps } from './field';
import { isFocusVisibleRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

const groupStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex rounded-md border border-slate-200 text-md leading-5 text-slate-800`,
});

// Number field

export interface NumberFieldProps
  extends Omit<AriaNumberFieldProps, 'children'>,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'group', Omit<AriaGroupProps, 'children'>>,
    MixinProps<'button', Omit<AriaButtonProps, 'children' | 'slot'>>,
    MixinProps<'input', Omit<AriaInputProps, 'children'>> {
  ref?: Ref<HTMLDivElement>;
  label?: FieldLabelProps['children'];
}

export const NumberField = ({
  ref,
  label,
  groupClassName,
  buttonClassName,
  inputClassName,
  ...mixProps
}: NumberFieldProps) => {
  const props = splitProps(mixProps, 'label', 'group', 'button', 'input');

  return (
    <AriaNumberField ref={ref} {...props.rest}>
      {label && <FieldLabel {...props.label}>{label}</FieldLabel>}

      <AriaGroup className={composeRenderPropsTV(groupClassName, groupStyles)} {...props.group}>
        <AriaButton
          slot='decrement'
          className={composeRenderPropsTW(
            buttonClassName,
            tw`flex size-8 items-center justify-center border-r border-slate-200`,
          )}
          {...props.button}
        >
          <FiMinus />
        </AriaButton>

        <AriaInput className={composeRenderPropsTW(inputClassName, tw`flex-1 px-3 outline-none`)} {...props.input} />

        <AriaButton
          slot='increment'
          className={composeRenderPropsTW(
            buttonClassName,
            tw`flex size-8 items-center justify-center border-l border-slate-200`,
          )}
          {...props.button}
        >
          <FiPlus />
        </AriaButton>
      </AriaGroup>
    </AriaNumberField>
  );
};

// Number field RHF wrapper

export interface NumberFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<NumberFieldProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const NumberFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: NumberFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: NumberFieldProps = {
    name: field.name,
    value: field.value,
    onChange: field.onChange,
    onBlur: field.onBlur,
    isDisabled: field.disabled ?? false,
    validationBehavior: 'aria',
    isInvalid: fieldState.invalid,
  };

  return <NumberField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
