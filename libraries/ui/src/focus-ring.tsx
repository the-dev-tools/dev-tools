import { tv } from 'tailwind-variants';

import { tw } from './tailwind-literal';

const focusedStyles = { true: tw`outline-4 outline-violet-200` };

const baseStyles = tv({
  base: tw`relative outline outline-0 outline-transparent transition-colors`,
});

export const focusRingStyles = tv({
  extend: baseStyles,
  variants: {
    isFocused: focusedStyles,
    isFocusWithin: focusedStyles,
  },
});

export const isFocusVisibleRingStyles = tv({ extend: baseStyles, variants: { isFocusVisible: focusedStyles } });
export const isFocusVisibleRingRenderPropKeys = ['isFocusVisible'] as const;
