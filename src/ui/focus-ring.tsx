import { tv } from 'tailwind-variants';

import { tw } from '@/utils';

const focusStyles = {
  false: tw`outline-0`,
  true: tw`!border-indigo-400 !outline-4 !outline-indigo-200`,
};

export const styles = tv({
  base: tw`outline border-transparent outline-transparent transition-[border-color,outline-color,outline-width]`,
  variants: {
    isFocused: focusStyles,
    isFocusWithin: focusStyles,
  },
});
