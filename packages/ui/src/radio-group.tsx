import { Struct } from 'effect';
import { ComponentProps } from 'react';
import {
  Radio as AriaRadio,
  RadioGroup as AriaRadioGroup,
  RadioGroupProps as AriaRadioGroupProps,
  RadioProps as AriaRadioProps,
  composeRenderProps,
} from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';

import { FieldError, FieldErrorProps, FieldLabel, FieldLabelProps } from './field';
import { isFocusVisibleRingStyles } from './focus-ring';
import { MixinProps, splitProps } from './mixin-props';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Group

const containerStyles = tv({
  base: tw`flex`,
  variants: {
    orientation: {
      horizontal: tw`gap-3`,
      vertical: tw`flex-col`,
    },
  },
  defaultVariants: {
    orientation: 'vertical',
  },
});

export interface RadioGroupProps
  extends AriaRadioGroupProps,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'container', Omit<ComponentProps<'div'>, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
}

export const RadioGroup = ({ children, className, containerClassName, error, label, ...props }: RadioGroupProps) => {
  const forwardedProps = splitProps(props, 'label', 'container', 'error');

  return (
    <AriaRadioGroup {...forwardedProps.rest} className={composeRenderPropsTW(className, tw`group flex flex-col gap-2`)}>
      {composeRenderProps(children, (children, renderProps) => (
        <>
          {label && <FieldLabel {...forwardedProps.label}>{label}</FieldLabel>}
          <div
            {...forwardedProps.container}
            className={containerStyles({
              ...Struct.pick(renderProps, ...containerStyles.variantKeys),
              className: containerClassName,
            })}
          >
            {children}
          </div>
          <FieldError {...forwardedProps.error}>{error}</FieldError>
        </>
      ))}
    </AriaRadioGroup>
  );
};

// Item

const itemStyles = tv({
  base: tw`group flex cursor-pointer items-center gap-1.5 text-md leading-5 font-medium tracking-tight text-slate-800`,
  variants: {
    isDisabled: { true: tw`text-gray-300` },
  },
});

const indicatorStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`size-4 rounded-full border`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isDisabled: { true: tw`border-slate-200 bg-slate-200` },
    isInvalid: { true: tw`border-red-700 bg-red-700` },
    isPressed: { true: null },
    isSelected: {
      false: tw`border-slate-200 bg-white`,
      true: tw`border-violet-600 bg-violet-600`,
    },
  },
  compoundVariants: [
    { className: tw`border-slate-400`, isPressed: true, isSelected: false },
    { className: tw`border-red-800`, isInvalid: true, isPressed: true },
  ],
});

export interface RadioProps
  extends AriaRadioProps,
    MixinProps<'indicator', Omit<ComponentProps<'div'>, 'children'>>,
    MixinProps<'ring', Omit<ComponentProps<'div'>, 'children'>> {}

export const Radio = ({ children, className, indicatorClassName, ringClassName, ...props }: RadioProps) => {
  const forwardedProps = splitProps(props, 'indicator', 'ring');

  return (
    <AriaRadio {...forwardedProps.rest} className={composeRenderPropsTV(className, itemStyles)}>
      {composeRenderProps(children, (children, renderProps) => (
        <>
          <div
            className={indicatorStyles({ className: indicatorClassName, ...renderProps })}
            {...forwardedProps.indicator}
          >
            <div
              className={twMerge(tw`size-full rounded-full border-2 border-white`, ringClassName)}
              {...forwardedProps.ring}
            />
          </div>

          {children}
        </>
      ))}
    </AriaRadio>
  );
};
