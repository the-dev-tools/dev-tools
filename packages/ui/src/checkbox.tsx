import { Struct } from 'effect';
import { ComponentProps, ForwardedRef, forwardRef } from 'react';
import { mergeProps } from 'react-aria';
import {
  Checkbox as AriaCheckbox,
  CheckboxProps as AriaCheckboxProps,
  composeRenderProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { IconBaseProps } from 'react-icons';
import { LuCheck, LuMinus } from 'react-icons/lu';
import { twMerge } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { isFocusedRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Root

export const rootStyles = tv({
  base: tw`group flex items-center gap-2`,
  variants: {
    variant: {
      'table-cell': tw`p-1`,
    },
  },
});

export interface CheckboxRootProps extends AriaCheckboxProps, VariantProps<typeof rootStyles> {}

export const CheckboxRoot = forwardRef(
  ({ className, ...props }: CheckboxRootProps, ref: ForwardedRef<HTMLLabelElement>) => {
    const forwardedProps = Struct.omit(props, ...rootStyles.variantKeys);
    const variantProps = Struct.pick(props, ...rootStyles.variantKeys);

    return (
      <AriaCheckbox
        {...forwardedProps}
        ref={ref}
        className={composeRenderPropsTV(className, rootStyles, variantProps)}
      />
    );
  },
);
CheckboxRoot.displayName = 'CheckboxRoot';

// Box

export const checkboxBoxStyles = tv({
  extend: isFocusedRingStyles,
  base: tw`flex size-5 flex-none cursor-pointer items-center justify-center rounded border-2 border-black`,
});

export interface CheckboxBoxProps extends ComponentProps<'div'>, VariantProps<typeof checkboxBoxStyles> {}

export const CheckboxBox = ({ className, ...props }: CheckboxBoxProps) => {
  const forwardedProps = Struct.omit(props, ...checkboxBoxStyles.variantKeys);
  const variantProps = Struct.pick(props, ...checkboxBoxStyles.variantKeys);
  return <div {...forwardedProps} className={checkboxBoxStyles({ ...variantProps, className })} />;
};

// Indicator

export interface CheckboxIndicatorProps extends IconBaseProps {
  isIndeterminate?: boolean;
  isSelected?: boolean;
}

export const CheckboxIndicator = ({ isIndeterminate, isSelected, className, ...props }: CheckboxIndicatorProps) => {
  const forwardedClassName = twMerge(tw`size-4`, className);
  if (isIndeterminate) return <LuMinus {...props} className={forwardedClassName} />;
  if (isSelected) return <LuCheck {...props} className={forwardedClassName} />;
  return <div className={forwardedClassName} />;
};

// Mix

export interface CheckboxProps
  extends CheckboxRootProps,
    MixinProps<'box', CheckboxBoxProps>,
    MixinProps<'indicator', CheckboxIndicatorProps> {}

export const Checkbox = forwardRef(({ children, ...props }: CheckboxProps, ref: ForwardedRef<HTMLLabelElement>) => {
  const forwardedProps = splitProps(props, 'box', 'indicator');
  return (
    <CheckboxRoot {...forwardedProps.rest} ref={ref}>
      {composeRenderProps(children, (children, renderProps) => (
        <>
          <CheckboxBox {...Struct.pick(renderProps, ...checkboxBoxStyles.variantKeys)} {...forwardedProps.box}>
            <CheckboxIndicator
              {...Struct.pick(renderProps, 'isIndeterminate', 'isSelected')}
              {...forwardedProps.indicator}
            />
          </CheckboxBox>
          {children}
        </>
      ))}
    </CheckboxRoot>
  );
});
Checkbox.displayName = 'Checkbox';

// RHF wrapper mix

export interface CheckboxRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<CheckboxProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const CheckboxRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: CheckboxRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: false as never, ...controllerProps });

  const fieldProps: CheckboxProps = {
    name: field.name,
    isSelected: field.value,
    onChange: field.onChange,
    onBlur: field.onBlur,
    isDisabled: field.disabled ?? false,
    validationBehavior: 'aria',
    isInvalid: fieldState.invalid,
  };

  return <Checkbox {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
