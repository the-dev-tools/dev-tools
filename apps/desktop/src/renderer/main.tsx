import { ConfigProvider, Layer, pipe, Record } from 'effect';
import { createRoot } from 'react-dom/client';
import { addGlobalLayer, App } from '@the-dev-tools/client';
import packageJson from '../../package.json';

pipe(
  {
    ...import.meta.env,
    VERSION: packageJson.version,
  },
  Record.mapKeys((_) => _.replaceAll('__', '.')),
  Record.toEntries,
  (_) => new Map(_ as [string, string][]),
  ConfigProvider.fromMap,
  Layer.setConfigProvider,
  addGlobalLayer,
);

const root = createRoot(document.getElementById('root')!);
window.electron.onClose(() => void root.unmount());
root.render(<App finalizer={() => void window.electron.onCloseDone()} />);
