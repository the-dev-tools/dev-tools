import { StorybookConfig } from '@storybook/react-vite';

const config: StorybookConfig = {
  addons: ['@storybook/addon-docs'],
  framework: {
    name: '@storybook/react-vite',
    options: {},
  },
  stories: ['./Introduction.mdx'],

  refs: {
    ui: {
      title: 'UI',
      url: 'http://localhost:4401',
    },

    client: {
      title: 'Client',
      url: 'http://localhost:4402',
    },
  },
};

export default config;
