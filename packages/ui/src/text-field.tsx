import { pipe, Record, String, Struct } from 'effect';
import { forwardRef, useCallback, useState } from 'react';
import { mergeProps } from 'react-aria';
import {
  Input as AriaInput,
  type InputProps as AriaInputProps,
  TextArea as AriaTextArea,
  type TextAreaProps as AriaTextAreaProps,
  TextField as AriaTextField,
  type TextFieldProps as AriaTextFieldProps,
  composeRenderProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv, VariantProps } from 'tailwind-variants';

import { FieldError, type FieldErrorProps, FieldLabel, type FieldLabelProps } from './field';
import { isFocusVisibleRingStyles } from './focus-ring';
import { type MixinProps, splitProps } from './mixin-props';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Input

export const inputStyles = tv({
  base: tw`text-md rounded-md border border-slate-200 px-3 py-1.5 leading-5 text-slate-800`,
  extend: isFocusVisibleRingStyles,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isDisabled: {
      true: tw`bg-slate-100 opacity-50`,
    },
    variant: {
      'table-cell': tw`w-full min-w-0 rounded-none border-transparent px-5 py-1.5 -outline-offset-4`,
    },
  },
});

const inputVariantKeys = pipe(
  Struct.omit(inputStyles.variants, ...isFocusVisibleRingStyles.variantKeys, 'isDisabled'),
  Record.keys,
);

// Editable text state

export interface UseEditableTextStateProps {
  onSuccess: (value: string) => Promise<unknown>;
  value: string;
}

export const useEditableTextState = ({ onSuccess, value }: UseEditableTextStateProps) => {
  const [isEditing, setIsEditing] = useState(false);

  const edit = useCallback(() => void setIsEditing(true), []);

  const onBlur = useCallback(
    async (event: React.FocusEvent<HTMLInputElement>) => {
      await onSuccess(event.currentTarget.value);
      setIsEditing(false);
    },
    [onSuccess],
  );

  const onKeyDown = useCallback(
    async (event: React.KeyboardEvent<HTMLInputElement>) => {
      if (event.key === 'Enter') await onSuccess(event.currentTarget.value);
      if (['Enter', 'Escape'].includes(event.key)) setIsEditing(false);
    },
    [onSuccess],
  );

  return {
    edit,
    isEditing,
    textFieldProps: {
      autoFocus: true,
      defaultValue: value,
      inputOnBlur: onBlur,
      inputOnKeyDown: onKeyDown,
    } satisfies TextFieldProps,
  };
};

// Root

interface RootProps
  extends AriaTextFieldProps,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
}

const Root = ({ children, className, error, label, ...props }: RootProps) => {
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
  extends MixinProps<'input', Omit<AriaInputProps, 'children'>>,
    Omit<RootProps, 'children'>,
    VariantProps<typeof inputStyles> {}

export const TextField = forwardRef(
  ({ inputClassName, ...props }: TextFieldProps, ref: React.ForwardedRef<HTMLInputElement>) => {
    const forwardedProps = splitProps(props, 'input');

    const rootForwardedProps = Struct.omit(forwardedProps.rest, ...inputVariantKeys);
    const variantProps = Struct.pick(forwardedProps.rest, ...inputVariantKeys);

    return (
      <Root {...rootForwardedProps}>
        <AriaInput
          {...forwardedProps.input}
          className={composeRenderPropsTV(inputClassName, inputStyles, variantProps)}
          ref={ref}
        />
      </Root>
    );
  },
);
TextField.displayName = 'TextField';

// Text field RHF wrapper

export interface TextFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<TextFieldProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const TextFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: TextFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: TextFieldProps = {
    error: fieldState.error?.message,
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
    value: field.value,
  };

  return <TextField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};

// Text area field

export interface TextAreaFieldProps
  extends MixinProps<'area', Omit<AriaTextAreaProps, 'children'>>,
    Omit<RootProps, 'children'>,
    VariantProps<typeof inputStyles> {}

export const TextAreaField = forwardRef(
  ({ areaClassName, ...props }: TextAreaFieldProps, ref: React.ForwardedRef<HTMLTextAreaElement>) => {
    const forwardedProps = splitProps(props, 'area');

    const rootForwardedProps = Struct.omit(forwardedProps.rest, ...inputStyles.variantKeys);
    const variantProps = Struct.pick(forwardedProps.rest, ...inputStyles.variantKeys);

    return (
      <Root {...rootForwardedProps}>
        <AriaTextArea
          {...forwardedProps.area}
          className={composeRenderPropsTV(areaClassName, inputStyles, variantProps)}
          ref={ref}
        />
      </Root>
    );
  },
);
TextAreaField.displayName = 'TextAreaField';

// Text area field RHF wrapper

export interface TextAreaFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<TextAreaFieldProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const TextAreaFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: TextAreaFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: TextFieldProps = {
    error: fieldState.error?.message,
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
    value: field.value,
  };

  return <TextAreaField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
