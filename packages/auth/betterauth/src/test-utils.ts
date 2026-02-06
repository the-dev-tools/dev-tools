import { createClient } from "@libsql/client";

import { type AuthConfig, createAuth } from "./auth.js";
import { initDatabase } from "./db.js";
import { createInternalAuthService, type ServiceConfig } from "./service.js";

const defaultServiceConfig: ServiceConfig = {
  jwt: { accessTokenExpiry: 15 * 60 },
  refreshToken: { expiry: 7 * 24 * 60 * 60 },
};

const defaultAuthConfig: AuthConfig = {
  baseURL: "http://localhost:50051",
  oauth: {
    github: { clientId: "test-github-id", clientSecret: "test-github-secret" },
    google: { clientId: "test-google-id", clientSecret: "test-google-secret" },
    microsoft: {
      clientId: "test-microsoft-id",
      clientSecret: "test-microsoft-secret",
    },
  },
  secret: "test-auth-secret",
};

export async function createTestService(overrides?: {
  authConfig?: Partial<AuthConfig>;
  serviceConfig?: Partial<ServiceConfig>;
}) {
  const rawDb = createClient({ url: ":memory:" });
  const authConfig = { ...defaultAuthConfig, ...overrides?.authConfig };
  const auth = createAuth(rawDb, authConfig);

  await initDatabase(rawDb);

  const jwtSecret = new TextEncoder().encode("test-jwt-secret");
  const serviceConfig = { ...defaultServiceConfig, ...overrides?.serviceConfig };

  const service = createInternalAuthService({
    auth,
    config: serviceConfig,
    jwtSecret,
    rawDb,
  });

  return { auth, config: serviceConfig, jwtSecret, rawDb, service };
}
