import path from 'path';
import { Array, pipe, String } from 'effect';
import { ContentConfig } from 'tailwindcss/types/config';

export const tailwindContent = pipe(
  '@the-dev-tools/ui',
  require.resolve,
  (_) => path.dirname(_),
  String.concat('/**/*.tsx'),
  (_) => path.join(_),
  Array.ensure,
) satisfies ContentConfig;
