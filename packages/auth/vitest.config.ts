import type { UserConfig } from 'vitest/config';

export default {
  test: {
    include: ['src/**/*.test.ts'],
  },
} satisfies UserConfig;
