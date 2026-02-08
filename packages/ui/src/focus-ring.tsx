import { tv } from 'tailwind-variants';
import { tw } from './tailwind-literal';

const baseStyles = tv({ base: tw`relative outline-0 outline-transparent transition-colors` });

export const focusRingStyles = tv({ extend: baseStyles, base: tw`focus:outline-4 focus:outline-ring` });

export const focusVisibleRingStyles = tv({
  extend: baseStyles,
  base: tw`focus-visible:outline-4 focus-visible:outline-ring`,
});

export const focusWithinRingStyles = tv({
  extend: baseStyles,
  base: tw`focus-within:outline-4 focus-within:outline-ring`,
});

export const focusVisibleWithinRingStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`has-focus-visible:outline-4 has-focus-visible:outline-ring`,
});
