import { Struct } from 'effect';
import { RefAttributes, useCallback, useState } from 'react';
import { mergeProps } from 'react-aria';
import * as RAC from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv, VariantProps } from 'tailwind-variants';
import { FieldError, type FieldErrorProps, FieldLabel, type FieldLabelProps } from './field';
import { focusVisibleRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps, composeTailwindRenderProps } from './utils';

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
      onBlur,
      onKeyDown,
    } satisfies TextFieldProps,
  };
};

// Text Field

export interface TextFieldProps extends RAC.TextFieldProps {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
}

export const TextField = ({ children, className, error, label, ...props }: TextFieldProps) => (
  <RAC.TextField
    {...props}
    {...(!label && !props['aria-label'] && props.name && { 'aria-label': 'String.capitalize(props.name)' })}
    className={composeTailwindRenderProps(className, tw`flex flex-col gap-1`)}
  >
    {RAC.composeRenderProps(children, (children) => (
      <>
        {label && <FieldLabel>{label}</FieldLabel>}
        {children}
        <FieldError>{error}</FieldError>
      </>
    ))}
  </RAC.TextField>
);

// Text input field

export const textInputFieldStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`rounded-md border border-slate-200 px-3 py-1.5 text-md leading-5 text-slate-800`,
  variants: {
    isTableCell: {
      false: tw`disabled:bg-slate-100 disabled:opacity-50`,
      true: tw`w-full min-w-0 rounded-none border-transparent px-5 py-1.5 -outline-offset-4`,
    },
  },
});

export interface TextInputFieldProps
  extends Omit<TextFieldProps, 'children'>,
    RefAttributes<HTMLInputElement>,
    VariantProps<typeof textInputFieldStyles> {
  inputClassName?: RAC.InputProps['className'];
  placeholder?: RAC.InputProps['placeholder'];
}

export const TextInputField = ({ className = '', inputClassName, placeholder, ref, ...props }: TextInputFieldProps) => (
  <TextField {...props} className={className}>
    <RAC.Input
      className={composeStyleRenderProps(inputClassName, textInputFieldStyles)}
      placeholder={placeholder}
      ref={ref}
    />
  </TextField>
);

// Text input field RHF wrapper

export interface TextInputFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<TextInputFieldProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const TextInputFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: TextInputFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: TextInputFieldProps = {
    error: fieldState.error?.message,
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
    value: field.value,
  };

  return <TextInputField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};

// Text area field

export interface TextAreaFieldProps
  extends Omit<TextFieldProps, 'children'>,
    RefAttributes<HTMLTextAreaElement>,
    VariantProps<typeof textInputFieldStyles> {}

export const TextAreaField = ({ className = '', ref, ...props }: TextAreaFieldProps) => (
  <TextField {...props} className={className}>
    <RAC.TextArea className={textInputFieldStyles(props)} ref={ref} />
  </TextField>
);

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

  const fieldProps: TextInputFieldProps = {
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
