import { Result, useAtomValue } from '@effect-atom/atom-react';
import { authClient } from '@the-dev-tools/auth';
import { runtimeAtom } from '../lib';

const authAtom = runtimeAtom.atom(authClient);
export const useAuth = () => useAtomValue(authAtom).pipe(Result.getOrThrow);
