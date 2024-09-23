import { String, Struct } from 'effect';
import { forwardRef } from 'react';
import {
  Input as AriaInput,
  TextArea as AriaTextArea,
  TextField as AriaTextField,
  composeRenderProps,
  type InputProps as AriaInputProps,
  type TextAreaProps as AriaTextAreaProps,
  type TextFieldProps as AriaTextFieldProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { FieldError, FieldLabel, type FieldErrorProps, type FieldLabelProps } from './field';
import { focusRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Input

const inputStyles = tv({
  extend: focusRingStyles,
  base: tw`col-start-2 rounded border border-black px-2 py-1 rac-invalid:border-red-600`,
});

// Root

interface RootProps
  extends AriaTextFieldProps,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  label?: FieldLabelProps['children'];
  error?: FieldErrorProps['children'];
}

const Root = ({ className, children, label, error, ...props }: RootProps) => {
  const forwardedProps = splitProps(props, 'label', 'error');

  if (!label && !forwardedProps.rest['aria-label'] && forwardedProps.rest.name) {
    forwardedProps.rest['aria-label'] = String.capitalize(forwardedProps.rest.name);
  }

  return (
    <AriaTextField {...forwardedProps.rest} className={composeRenderPropsTW(className, tw`flex flex-col gap-1`)}>
      {composeRenderProps(children, (children) => (
        <>
          {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
          {children}
          <FieldError {...forwardedProps.error}>{error}</FieldError>
        </>
      ))}
    </AriaTextField>
  );
};

// Text field

export interface TextFieldProps
  extends Omit<RootProps, 'children'>,
    MixinProps<'input', Omit<AriaInputProps, 'children'>> {}

export const TextField = forwardRef(
  ({ inputClassName, ...props }: TextFieldProps, ref: React.ForwardedRef<HTMLInputElement>) => {
    const forwardedProps = splitProps(props, 'input');

    return (
      <Root {...forwardedProps.rest}>
        <AriaInput {...forwardedProps} ref={ref} className={composeRenderPropsTV(inputClassName, inputStyles)} />
      </Root>
    );
  },
);
TextField.displayName = 'TextField';

// Text field RHF wrapper

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

// Text area field

export interface TextAreaFieldProps
  extends Omit<RootProps, 'children'>,
    MixinProps<'area', Omit<AriaTextAreaProps, 'children'>> {}

export const TextAreaField = forwardRef(
  ({ areaClassName, ...props }: TextAreaFieldProps, ref: React.ForwardedRef<HTMLTextAreaElement>) => {
    const forwardedProps = splitProps(props, 'area');

    return (
      <Root {...forwardedProps.rest}>
        <AriaTextArea {...forwardedProps.area} ref={ref} className={composeRenderPropsTV(areaClassName, inputStyles)} />
      </Root>
    );
  },
);
TextAreaField.displayName = 'TextAreaField';

// Text area field RHF wrapper

export interface TextAreaFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<
      TextAreaFieldProps,
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

export const TextAreaFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: TextAreaFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController(controllerProps);

  return (
    <TextAreaField
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
