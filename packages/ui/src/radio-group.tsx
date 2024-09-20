import { Struct } from 'effect';
import { ComponentProps } from 'react';
import {
  Radio as AriaRadio,
  RadioGroup as AriaRadioGroup,
  RadioGroupProps as AriaRadioGroupProps,
  RadioProps as AriaRadioProps,
  composeRenderProps,
} from 'react-aria-components';
import { tv } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { FieldError, FieldErrorProps, FieldLabel, FieldLabelProps } from './field';
import { isFocusedRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Group

const containerStyles = tv({
  base: tw`flex gap-2`,
  variants: {
    orientation: {
      vertical: tw`flex-col`,
      horizontal: tw`gap-4`,
    },
  },
});

export interface RadioGroupProps
  extends AriaRadioGroupProps,
    MixinProps<'label', Omit<FieldLabelProps, 'children'>>,
    MixinProps<'container', Omit<ComponentProps<'div'>, 'children'>>,
    MixinProps<'error', Omit<FieldErrorProps, 'children'>> {
  label?: FieldLabelProps['children'];
  error?: FieldErrorProps['children'];
}

export const RadioGroup = ({ children, className, label, containerClassName, error, ...props }: RadioGroupProps) => {
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
  base: tw`group flex cursor-pointer items-center gap-1.5 text-sm leading-tight transition`,
  variants: {
    isDisabled: { true: tw`text-gray-300` },
  },
});

const indicatorStyles = tv({
  extend: isFocusedRingStyles,
  base: tw`size-4 rounded-full border-2 bg-white transition-all`,
  variants: {
    ...({} as typeof isFocusedRingStyles.variants),
    isSelected: {
      false: tw`border-gray-400`,
      true: tw`border-4 border-gray-700`,
    },
    isInvalid: { true: tw`border-red-700` },
    isDisabled: { true: tw`border-gray-200` },
    isPressed: { true: null },
  },
  compoundVariants: [
    { isSelected: false, isPressed: true, className: tw`border-gray-500` },
    { isSelected: true, isPressed: true, className: tw`border-gray-800` },
    { isInvalid: true, isPressed: true, className: tw`border-red-800` },
  ],
});

export interface RadioProps extends AriaRadioProps, MixinProps<'indicator', Omit<ComponentProps<'div'>, 'children'>> {}

export const Radio = ({ className, children, indicatorClassName, ...props }: RadioProps) => {
  const forwardedProps = splitProps(props, 'indicator');
  return (
    <AriaRadio {...forwardedProps.rest} className={composeRenderPropsTV(className, itemStyles)}>
      {composeRenderProps(children, (children, renderProps) => (
        <>
          <div
            {...forwardedProps.indicator}
            className={indicatorStyles({
              ...Struct.pick(renderProps, ...indicatorStyles.variantKeys),
              className: indicatorClassName,
            })}
          />
          {children}
        </>
      ))}
    </AriaRadio>
  );
};
