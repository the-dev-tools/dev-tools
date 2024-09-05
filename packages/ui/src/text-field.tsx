import { Struct } from 'effect';
import { forwardRef } from 'react';
import {
  Input as AriaInput,
  TextField as AriaTextField,
  type InputProps as AriaInputProps,
  type TextFieldProps as AriaTextFieldProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv, type VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { FieldError, FieldLabel, type FieldErrorProps, type FieldLabelProps } from './field';
import { focusRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Root

export interface TextFieldRootProps extends AriaTextFieldProps {}

export const TextFieldRoot = ({ className, ...props }: TextFieldRootProps) => (
  <AriaTextField {...props} className={composeRenderPropsTW(className, tw`flex flex-col gap-1`)} />
);

// Input

export const textFieldInputStyles = tv({
  extend: focusRingStyles,
  base: tw`col-start-2 rounded border border-black px-2 py-1 rac-invalid:border-red-600`,
});

export interface TextFieldInputProps extends AriaInputProps, VariantProps<typeof textFieldInputStyles> {}

export const TextFieldInput = forwardRef(
  ({ className, ...props }: TextFieldInputProps, ref: React.ForwardedRef<HTMLInputElement>) => {
    const forwardedProps = Struct.omit(props, ...textFieldInputStyles.variantKeys);
    const variantProps = Struct.pick(props, ...textFieldInputStyles.variantKeys);
    return (
      <AriaInput
        {...forwardedProps}
        ref={ref}
        className={composeRenderPropsTV(className, textFieldInputStyles, variantProps)}
      />
    );
  },
);
TextFieldInput.displayName = 'TextFieldInput';

// Mix

export interface TextFieldProps
  extends Omit<TextFieldRootProps, 'children'>,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'input', Omit<TextFieldInputProps, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  label?: FieldLabelProps['children'];
  error?: FieldErrorProps['children'];
}

export const TextField = forwardRef(
  ({ label, error, ...props }: TextFieldProps, ref: React.ForwardedRef<HTMLInputElement>) => {
    const forwardedProps = splitProps(props, 'label', 'input', 'error');
    return (
      <TextFieldRoot {...forwardedProps.rest}>
        {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
        <TextFieldInput {...forwardedProps.input} ref={ref} />
        <FieldError {...forwardedProps.error}>{error}</FieldError>
      </TextFieldRoot>
    );
  },
);
TextField.displayName = 'TextField';

// RHF wrapper mix

export interface TextFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<
      TextFieldProps,
      | ControllerPropKeys
      | 'name'
      | 'value'
      | 'onChange'
      | 'onBlur'
      | 'isDisabled'
      | 'validationBehavior'
      | 'isInvalid'
      | 'error'
    >,
    UseControllerProps<TFieldValues, TName> {}

export const TextFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: TextFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);
  const { field, fieldState } = useController(controllerProps);
  return (
    <TextField
      {...forwardedProps}
      ref={field.ref}
      name={field.name}
      value={field.value}
      onChange={field.onChange}
      onBlur={field.onBlur}
      isDisabled={field.disabled ?? false}
      validationBehavior='aria'
      isInvalid={fieldState.invalid}
      error={fieldState.error?.message}
    />
  );
};
