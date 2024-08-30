import { fileURLToPath } from 'node:url';
import { dirname, normalize } from 'path';
import { Array, pipe, String } from 'effect';
import { type ContentConfig } from 'tailwindcss/types/config';

export const tailwindContent = pipe(
  import.meta.url,
  fileURLToPath,
  dirname,
  String.concat('/**/*.tsx'),
  normalize,
  Array.make,
) satisfies ContentConfig;
