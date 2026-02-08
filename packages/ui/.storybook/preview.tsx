import { Preview } from '@storybook/react-vite';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { StrictMode, useEffect } from 'react';
import { UiProvider } from '../src/provider';

import '../src/styles.css';

const preview: Preview = {
  decorators: [
    (Story, context) => {
      const theme = context.globals['theme'] as string;

      useEffect(() => {
        document.documentElement.classList.toggle('dark', theme === 'dark');
        document.body.style.backgroundColor = theme === 'dark' ? '#0f172a' : '#ffffff';
      }, [theme]);

      const rootRoute = createRootRoute({ component: Story });
      const router = createRouter({ routeTree: rootRoute });

      let _ = <RouterProvider router={router} />;
      _ = <UiProvider>{_}</UiProvider>;
      _ = <StrictMode>{_}</StrictMode>;
      return _;
    },
  ],
  globalTypes: {
    theme: {
      description: 'Toggle dark mode',
      toolbar: {
        dynamicTitle: true,
        icon: 'mirror',
        items: [
          { title: 'Light', value: 'light' },
          { title: 'Dark', value: 'dark' },
        ],
      },
    },
  },
  initialGlobals: {
    theme: 'light',
  },
  parameters: {
    layout: 'centered',
  },
};

export default preview;
