import { dirname, join } from 'path';
import { Array, pipe, String } from 'effect';
import { ContentConfig } from 'tailwindcss/types/config';

export const tailwindContent = pipe(
  '@the-dev-tools/ui',
  require.resolve,
  dirname,
  String.concat('/**/*.tsx'),
  join,
  Array.ensure,
) satisfies ContentConfig;
