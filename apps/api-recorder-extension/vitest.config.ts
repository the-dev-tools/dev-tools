import * as path from 'node:path';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: { alias: [{ find: /^~\/?/, replacement: `${path.join(import.meta.dirname, 'src')}/` }] },
  test: {
    include: ['test/**/*.test.ts'],
  },
});
