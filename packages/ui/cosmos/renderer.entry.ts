import { mountDomRenderer } from 'react-cosmos-dom';

// eslint-disable-next-line import-x/no-unresolved
import * as mountArgs from './imports';

import '../src/fonts';
import './styles.css';

// eslint-disable-next-line @typescript-eslint/no-unsafe-argument
mountDomRenderer(mountArgs);
