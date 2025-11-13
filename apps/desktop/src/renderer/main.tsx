import { Layer, pipe } from 'effect';
import { createRoot } from 'react-dom/client';
import { addGlobalLayer, App, configProviderFromMetaEnv } from '@the-dev-tools/client';
import packageJson from '../../package.json';

pipe(configProviderFromMetaEnv({ VERSION: packageJson.version }), Layer.setConfigProvider, addGlobalLayer);

const root = createRoot(document.getElementById('root')!);
window.electron.onClose(() => void root.unmount());
root.render(<App finalizer={() => void window.electron.onCloseDone()} />);
