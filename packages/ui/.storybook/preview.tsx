import { Preview } from '@storybook/react-vite';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { Option, pipe, Record, String } from 'effect';
import { StrictMode } from 'react';
import { UiProvider } from '../src/provider';

import '../src/styles/index.css';

const theme = pipe(
  new URLSearchParams(window.location.search).get('globals') ?? '',
  String.split(';'),
  Record.fromIterableWith((_) => {
    const [key, value] = String.split(_, ':');
    return [key, value];
  }),
  Record.get('backgrounds.value'),
  Option.getOrUndefined,
);

if (theme === 'dark') document.documentElement.classList.add('dark');

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
