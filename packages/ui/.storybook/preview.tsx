import { Preview } from '@storybook/react-vite';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { AriaRouterProvider } from '../src/router';

import '../src/styles.css';

const preview: Preview = {
  decorators: [
    (Story) => {
      const rootRoute = createRootRoute({ component: Story });
      const router = createRouter({ routeTree: rootRoute });

      let _ = <RouterProvider router={router} />;
      _ = <AriaRouterProvider>{_}</AriaRouterProvider>;
      _ = <StrictMode>{_}</StrictMode>;
      return _;
    },
  ],
  parameters: {
    layout: 'centered',
  },
};

export default preview;
