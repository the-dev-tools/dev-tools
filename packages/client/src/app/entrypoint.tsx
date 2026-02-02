import { Layer, pipe } from 'effect';
import { createRoot } from 'react-dom/client';
import { addGlobalLayer, App, configProviderFromMetaEnv } from '.';

import './styles.css';

pipe(configProviderFromMetaEnv(), Layer.setConfigProvider, addGlobalLayer);

createRoot(document.getElementById('root')!).render(<App />);
