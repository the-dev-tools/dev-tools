import { Struct } from 'effect';
import { ComponentProps, ForwardedRef, forwardRef, SVGProps } from 'react';
import { mergeProps } from 'react-aria';
import {
  Checkbox as AriaCheckbox,
  CheckboxProps as AriaCheckboxProps,
  composeRenderProps,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv, VariantProps } from 'tailwind-variants';

import { isFocusVisibleRingStyles } from './focus-ring';
import { MixinProps, splitProps } from './mixin-props';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

const rootStyles = tv({
  base: tw`group flex items-center gap-2`,
  variants: {
    variant: {
      'table-cell': tw`justify-self-center p-1`,
    },
  },
});

const boxStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex size-4 flex-none cursor-pointer items-center justify-center rounded-sm border border-slate-200 bg-white p-0.5 text-white`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isIndeterminate: { true: tw`border-violet-600 bg-violet-600` },
    isSelected: { true: tw`border-violet-600 bg-violet-600` },
  },
});

// Checkbox

export interface CheckboxProps
  extends AriaCheckboxProps,
    MixinProps<'box', Omit<ComponentProps<'div'>, 'children'>>,
    MixinProps<'indicator', SVGProps<SVGSVGElement>>,
    VariantProps<typeof rootStyles> {}

export const Checkbox = forwardRef(
  ({ boxClassName, children, className, ...props }: CheckboxProps, ref: ForwardedRef<HTMLLabelElement>) => {
    const forwardedProps = splitProps(props, 'box', 'indicator');

    const rootForwardedProps = Struct.omit(forwardedProps.rest, ...rootStyles.variantKeys);
    const rootVariantProps = Struct.pick(forwardedProps.rest, ...rootStyles.variantKeys);

    return (
      <AriaCheckbox
        className={composeRenderPropsTV(className, rootStyles, rootVariantProps)}
        ref={ref}
        {...rootForwardedProps}
      >
        {composeRenderProps(children, (children, renderProps) => (
          <>
            <div className={boxStyles({ className: boxClassName, ...renderProps })} {...forwardedProps.box}>
              {renderProps.isIndeterminate && (
                <svg
                  fill='none'
                  height='1em'
                  viewBox='0 0 10 2'
                  width='1em'
                  xmlns='http://www.w3.org/2000/svg'
                  {...forwardedProps.indicator}
                >
                  <path
                    d='M1 1h8.315'
                    stroke='currentColor'
                    strokeLinecap='round'
                    strokeLinejoin='round'
                    strokeWidth={1.5}
                  />
                </svg>
              )}

              {renderProps.isSelected && (
                <svg
                  fill='none'
                  height='1em'
                  viewBox='0 0 10 8'
                  width='1em'
                  xmlns='http://www.w3.org/2000/svg'
                  {...forwardedProps.indicator}
                >
                  <path
                    d='m.833 4.183 2.778 3.15L9.167 1.5'
                    stroke='currentColor'
                    strokeLinecap='round'
                    strokeLinejoin='round'
                    strokeWidth={1.2}
                  />
                </svg>
              )}
            </div>

            {children}
          </>
        ))}
      </AriaCheckbox>
    );
  },
);
Checkbox.displayName = 'Checkbox';

// RHF wrapper

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
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    isSelected: field.value,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
  };

  return <Checkbox {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
