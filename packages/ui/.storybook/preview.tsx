import { Preview } from '@storybook/react-vite';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { UiProvider } from '../src/provider';

import '../src/styles.css';

const preview: Preview = {
  decorators: [
    (Story) => {
      const rootRoute = createRootRoute({ component: Story });
      const router = createRouter({ routeTree: rootRoute });

      let _ = <RouterProvider router={router} />;
      _ = <UiProvider>{_}</UiProvider>;
      _ = <StrictMode>{_}</StrictMode>;
      return _;
    },
  ],
  parameters: {
    layout: 'centered',
  },
};

export default preview;
