import { tv } from 'tailwind-variants';
import { tw } from './tailwind-literal';

const baseStyles = tv({ base: tw`relative outline-0 outline-transparent transition-colors` });

export const focusRingStyles = tv({ extend: baseStyles, base: tw`focus:outline-4 focus:outline-accent-lower` });

export const focusVisibleRingStyles = tv({
  extend: baseStyles,
  base: tw`focus-visible:outline-4 focus-visible:outline-accent-lower`,
});

export const focusWithinRingStyles = tv({
  extend: baseStyles,
  base: tw`focus-within:outline-4 focus-within:outline-accent-lower`,
});

export const focusVisibleWithinRingStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`has-focus-visible:outline-4 has-focus-visible:outline-accent-lower`,
});
