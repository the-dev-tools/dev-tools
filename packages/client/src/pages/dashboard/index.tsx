import { resolveRoutesTo } from '../../shared/lib/router';

export const resolveRoutesFrom = resolveRoutesTo(import.meta.dirname, 'routes');
