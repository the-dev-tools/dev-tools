import { Struct } from 'effect';
import {
  FieldError as AriaFieldError,
  FieldErrorProps as AriaFieldErrorProps,
  Input as AriaInput,
  InputProps as AriaInputProps,
  Label as AriaLabel,
  LabelProps as AriaLabelProps,
  TextField as AriaTextField,
  TextFieldProps as AriaTextFieldProps,
} from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// -----------------------------------------------------------------------------
// Compound Components
// -----------------------------------------------------------------------------

// Root

export interface TextFieldRootProps extends AriaTextFieldProps {}

export const TextFieldRoot = ({ className, ...props }: TextFieldRootProps) => (
  <AriaTextField {...props} className={composeRenderPropsTW(className, tw`flex flex-col gap-1`)} />
);

// Label

export interface TextFieldLabelProps extends AriaLabelProps {}

export const TextFieldLabel = (props: TextFieldLabelProps) => <AriaLabel {...props} />;

// Input

export const textFieldInputStyles = tv({
  extend: focusRingStyles,
  base: tw`col-start-2 rounded border border-black px-2 py-1 rac-invalid:border-red-600`,
});

export interface TextFieldInputProps extends AriaInputProps, VariantProps<typeof textFieldInputStyles> {}

export const TextFieldInput = ({ className, ...props }: TextFieldInputProps) => {
  const forwardedProps = Struct.omit(props, ...textFieldInputStyles.variantKeys);
  const variantProps = Struct.pick(props, ...textFieldInputStyles.variantKeys);
  return (
    <AriaInput {...forwardedProps} className={composeRenderPropsTV(className, textFieldInputStyles, variantProps)} />
  );
};

// Error

export interface TextFieldErrorProps extends AriaFieldErrorProps {}

export const TextFieldError = ({ className, ...props }: TextFieldErrorProps) => (
  <AriaFieldError {...props} className={composeRenderPropsTW(className, tw`text-red-700`)} />
);

// -----------------------------------------------------------------------------
// Mix Components
// -----------------------------------------------------------------------------

export interface TextFieldProps
  extends Omit<TextFieldRootProps, 'children'>,
    MixinProps<'label', Omit<TextFieldLabelProps, 'children'>>,
    MixinProps<'input', Omit<TextFieldInputProps, 'children'>>,
    MixinProps<'error', Omit<TextFieldErrorProps, 'children'>> {
  label?: TextFieldLabelProps['children'];
  error?: TextFieldErrorProps['children'];
}

export const TextField = ({ label, error, ...props }: TextFieldProps) => {
  const forwardedProps = splitProps(props, 'label', 'input', 'error');
  return (
    <TextFieldRoot {...forwardedProps.rest}>
      {label && <TextFieldLabel {...forwardedProps.label}>{label}</TextFieldLabel>}
      <TextFieldInput {...forwardedProps.input} />
      <TextFieldError {...forwardedProps.error}>{error}</TextFieldError>
    </TextFieldRoot>
  );
};
