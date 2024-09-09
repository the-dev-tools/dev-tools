import { Struct } from 'effect';
import React from 'react';
import {
  Checkbox as AriaCheckbox,
  CheckboxProps as AriaCheckboxProps,
  composeRenderProps,
} from 'react-aria-components';
import { IconBaseProps } from 'react-icons';
import { LuCheck, LuMinus } from 'react-icons/lu';
import { tv, VariantProps } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { isFocusedRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTW } from './utils';

// Root

export interface CheckboxRootProps extends AriaCheckboxProps {}

export const CheckboxRoot = ({ className, ...props }: CheckboxRootProps) => (
  <AriaCheckbox {...props} className={composeRenderPropsTW(className, tw`group flex items-center gap-2`)} />
);

// Box

export const checkboxBoxStyles = tv({
  extend: isFocusedRingStyles,
  base: tw`flex size-5 flex-none items-center justify-center rounded border-2 border-black`,
});

export interface CheckboxBoxProps extends React.ComponentProps<'div'>, VariantProps<typeof checkboxBoxStyles> {}

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

export const CheckboxIndicator = ({ isIndeterminate, isSelected, ...props }: CheckboxIndicatorProps) => {
  if (isIndeterminate) return <LuMinus {...props} />;
  if (isSelected) return <LuCheck {...props} />;
  return null;
};

// Mix

export interface CheckboxProps
  extends CheckboxRootProps,
    MixinProps<'box', CheckboxBoxProps>,
    MixinProps<'indicator', CheckboxIndicatorProps> {}

export const Checkbox = ({ children, ...props }: CheckboxProps) => {
  const forwardedProps = splitProps(props, 'box', 'indicator');
  return (
    <CheckboxRoot {...forwardedProps.rest}>
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
};
