import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { betterAuth } from 'better-auth';
import { bearer } from 'better-auth/plugins';
import { createConnectTransport } from '@connectrpc/connect-node';

import { createRpcAdapter } from './rpc-adapter.js';
import { startGoServer, type GoServer } from './go-server.js';

// No jwt() plugin — avoids the jwks table which the Go authadapter marks as
// CodeUnimplemented. Tests cover user, session, and account tables only.
describe('BetterAuth → Go AuthAdapterService e2e', () => {
  let server: GoServer;
  let auth: ReturnType<typeof betterAuth>;

  beforeAll(async () => {
    server = await startGoServer();

    // Connect via Unix socket using HTTP/1.1 Connect protocol.
    const transport = createConnectTransport({
      baseUrl: 'http://localhost',
      httpVersion: '1.1',
      nodeOptions: { socketPath: server.socketPath },
    });

    auth = betterAuth({
      database: createRpcAdapter(transport),
      emailAndPassword: { enabled: true, requireEmailVerification: false },
      plugins: [bearer()],
      secret: 'test-secret-must-be-at-least-32-chars!!',
      baseURL: 'http://localhost',
    });
  });

  afterAll(() => {
    server?.kill();
  });

  it('creates a user and session via signUpEmail', async () => {
    const result = await auth.api.signUpEmail({
      body: { email: 'alice@example.com', password: 'password123', name: 'Alice' },
    });
    expect(result.user.email).toBe('alice@example.com');
    expect(result.user.id).toBeTruthy();
    expect(result.token).toBeTruthy();
  });

  it('signs in with correct credentials and gets a new session', async () => {
    await auth.api.signUpEmail({
      body: { email: 'bob@example.com', password: 'password456', name: 'Bob' },
    });
    const result = await auth.api.signInEmail({
      body: { email: 'bob@example.com', password: 'password456' },
    });
    expect(result.user.email).toBe('bob@example.com');
    expect(result.token).toBeTruthy();
  });

  it('rejects sign in with wrong password', async () => {
    await auth.api.signUpEmail({
      body: { email: 'carol@example.com', password: 'correct-pw!', name: 'Carol' },
    });
    await expect(
      auth.api.signInEmail({ body: { email: 'carol@example.com', password: 'wrongpass' } }),
    ).rejects.toThrow();
  });

  it('validates session token via getSession', async () => {
    const signup = await auth.api.signUpEmail({
      body: { email: 'dave@example.com', password: 'password123', name: 'Dave' },
    });
    const session = await auth.api.getSession({
      headers: new Headers({ authorization: `Bearer ${signup.token}` }),
    });
    expect(session?.user.email).toBe('dave@example.com');
  });

  it('revokes session and subsequent getSession returns null', async () => {
    const signup = await auth.api.signUpEmail({
      body: { email: 'eve@example.com', password: 'password123', name: 'Eve' },
    });
    await auth.api.revokeSession({
      body: { token: signup.token! },
      headers: new Headers({ authorization: `Bearer ${signup.token}` }),
    });
    const session = await auth.api.getSession({
      headers: new Headers({ authorization: `Bearer ${signup.token}` }),
    });
    expect(session).toBeNull();
  });

  it('rejects duplicate email on signUp', async () => {
    await auth.api.signUpEmail({
      body: { email: 'dup@example.com', password: 'password123', name: 'Dup' },
    });
    await expect(
      auth.api.signUpEmail({
        body: { email: 'dup@example.com', password: 'password456', name: 'Dup2' },
      }),
    ).rejects.toThrow();
  });
});
