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

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { isFocusVisibleRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

const rootStyles = tv({
  base: tw`group flex items-center gap-2`,
  variants: {
    variant: {
      'table-cell': tw`p-1`,
    },
  },
});

const boxStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex size-4 flex-none cursor-pointer items-center justify-center rounded border p-0.5 text-white transition-colors`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isSelected: {
      false: tw`border-slate-200 bg-white`,
      true: tw`border-violet-600 bg-violet-600`,
    },
  },
});

// Checkbox

export interface CheckboxProps
  extends AriaCheckboxProps,
    VariantProps<typeof rootStyles>,
    MixinProps<'box', Omit<ComponentProps<'div'>, 'children'>>,
    MixinProps<'indicator', SVGProps<SVGSVGElement>> {}

export const Checkbox = forwardRef(
  ({ className, children, boxClassName, ...props }: CheckboxProps, ref: ForwardedRef<HTMLLabelElement>) => {
    const forwardedProps = splitProps(props, 'box', 'indicator');

    const rootForwardedProps = Struct.omit(forwardedProps.rest, ...rootStyles.variantKeys);
    const rootVariantProps = Struct.pick(forwardedProps.rest, ...rootStyles.variantKeys);

    return (
      <AriaCheckbox
        ref={ref}
        className={composeRenderPropsTV(className, rootStyles, rootVariantProps)}
        {...rootForwardedProps}
      >
        {composeRenderProps(children, (children, renderProps) => (
          <>
            <div className={boxStyles({ className: boxClassName, ...renderProps })} {...forwardedProps.box}>
              {renderProps.isSelected && (
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  width='1em'
                  height='1em'
                  fill='none'
                  viewBox='0 0 10 8'
                  {...forwardedProps.indicator}
                >
                  <path
                    stroke='currentColor'
                    strokeLinecap='round'
                    strokeLinejoin='round'
                    strokeWidth={1.2}
                    d='m.833 4.183 2.778 3.15L9.167 1.5'
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
