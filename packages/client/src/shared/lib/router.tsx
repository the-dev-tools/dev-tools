import { defineVirtualSubtreeConfig, physical } from '@tanstack/virtual-file-routes';
import { relative, resolve } from 'node:path';

export const resolveRoutesTo =
  (...targetPaths: string[]) =>
  (...sourcePaths: string[]) => {
    const source = resolve(...sourcePaths);
    const target = resolve(...targetPaths);
    const path = relative(source, target);
    return defineVirtualSubtreeConfig([physical(path)]);
  };
