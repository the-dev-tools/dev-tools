import { tv } from 'tailwind-variants';

import { tw } from './tailwind-literal';

const isFocusedStyles = {
  true: tw`z-10 border-indigo-400 outline-4 outline-indigo-200`,
};

export const focusRingStyles = tv({
  base: tw`relative z-0 border-transparent outline outline-0 outline-transparent transition-[border-color,outline-color,outline-width]`,
  variants: {
    isFocused: isFocusedStyles,
    isFocusWithin: isFocusedStyles,
  },
});
