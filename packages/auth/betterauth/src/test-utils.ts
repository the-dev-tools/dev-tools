import { createClient } from '@libsql/client';
import { type AuthConfig, createAuth } from './auth.js';
import { initDatabase } from './db.js';
import { createInternalAuthService } from './service.js';

const defaultAuthConfig: AuthConfig = {
  baseURL: 'http://localhost:50051',
  oauth: {
    google: { clientId: 'test-google-id', clientSecret: 'test-google-secret' },
  },
  secret: 'test-auth-secret',
};

export async function createTestService(overrides?: { authConfig?: Partial<AuthConfig> }) {
  const rawDb = createClient({ url: ':memory:' });
  const authConfig = { ...defaultAuthConfig, ...overrides?.authConfig };
  const auth = createAuth(rawDb, authConfig);

  await initDatabase(rawDb);

  const service = createInternalAuthService({ auth, rawDb });

  return { auth, rawDb, service };
}
