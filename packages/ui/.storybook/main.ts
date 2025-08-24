import { StorybookConfig } from '@storybook/react-vite';
import { Array, pipe, String } from 'effect';
import { mergeConfig } from 'vite';

const config: StorybookConfig = {
  addons: ['@storybook/addon-docs'],
  framework: {
    name: '@storybook/react-vite',
    options: {},
  },
  stories: ['../src/**/*.@(mdx|stories.@(js|jsx|ts|tsx))'],

  typescript: {
    reactDocgen: 'react-docgen-typescript',
    reactDocgenTypescriptOptions: {
      EXPERIMENTAL_useProjectService: true,
      propFilter: ({ parent }) => {
        if (!parent) return true;
        return ['@types', '@react-types', 'typescript', '@tanstack/react-router', '@tanstack/router-core'].every(
          (_) => !parent.fileName.includes(`node_modules/${_}`),
        );
      },
      shouldExtractLiteralValuesFromEnum: true,
    },
  },

  viteFinal: async (config) => {
    const { default: tailwind } = await import('@tailwindcss/vite');
    const { default: react } = await import('@vitejs/plugin-react');
    const { nxViteTsPaths } = await import('@nx/vite/plugins/nx-tsconfig-paths.plugin');

    return mergeConfig(config, {
      plugins: [tailwind(), react({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }), nxViteTsPaths()],
    });
  },

  experimental_indexers: (indexers) =>
    (indexers ?? []).map((indexer) => ({
      ...indexer,
      createIndex: async (fileName, options) =>
        pipe(
          await indexer.createIndex(fileName, options),
          Array.map(({ __id, ...index }) => {
            const parts = pipe(options.makeTitle(index.title), String.split('.'), Array.map(kebabToHuman));

            let name = index.name ?? index.exportName;
            if (name === 'Default') name = Array.lastNonEmpty(parts);

            const title = ['UI', ...parts].join('/');

            return { ...index, name, title };
          }),
        ),
    })),
};

export default config;

const kebabToHuman = (self: string) => {
  let str = self[0]!.toUpperCase();
  for (let i = 1; i < self.length; i++) {
    str += self[i] === '-' ? ' ' + self[++i]!.toUpperCase() : self[i]!;
  }
  return str;
};
