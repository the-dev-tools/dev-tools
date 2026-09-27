import '~styles.css';

import * as React from 'react';
import { createRoot } from 'react-dom/client';

export const mount = (Page: React.ComponentType) => {
  const root = document.getElementById('root');
  if (!root) throw new Error('Missing #root element');
  createRoot(root).render(
    <React.StrictMode>
      <Page />
    </React.StrictMode>,
  );
};
