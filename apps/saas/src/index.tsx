import * as React from 'react';
import * as ReactDOM from 'react-dom/client';

import './styles.css';
import '@the-dev-tools/ui/fonts';

const rootEl = document.getElementById('root');
if (rootEl) {
  const root = ReactDOM.createRoot(rootEl);
  root.render(
    <React.StrictMode>
      <main>Welcome to The Dev Tools</main>
    </React.StrictMode>,
  );
}
