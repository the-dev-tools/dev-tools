import { Struct } from 'effect';
import {
  Input as AriaInput,
  TextField as AriaTextField,
  type InputProps as AriaInputProps,
  type TextFieldProps as AriaTextFieldProps,
} from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { FieldError, FieldLabel, type FieldErrorProps, type FieldLabelProps } from './field';
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

// -----------------------------------------------------------------------------
// Mix Components
// -----------------------------------------------------------------------------

export interface TextFieldProps
  extends Omit<TextFieldRootProps, 'children'>,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'input', Omit<TextFieldInputProps, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  label?: FieldLabelProps['children'];
  error?: FieldErrorProps['children'];
}

export const TextField = ({ label, error, ...props }: TextFieldProps) => {
  const forwardedProps = splitProps(props, 'label', 'input', 'error');
  return (
    <TextFieldRoot {...forwardedProps.rest}>
      {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
      <TextFieldInput {...forwardedProps.input} />
      <FieldError {...forwardedProps.error}>{error}</FieldError>
    </TextFieldRoot>
  );
};
