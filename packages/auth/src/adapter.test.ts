import { Command, FileSystem, Path, Url } from '@effect/platform';
import { NodeContext } from '@effect/platform-node';
import { runAdapterTest } from 'better-auth/adapters/test';
import { Effect, Layer, ManagedRuntime, pipe, Schedule } from 'effect';
import os from 'node:os';
import { afterAll, beforeAll, describe } from 'vitest';
import { HealthService } from '@the-dev-tools/spec/buf/api/health/v1/health_pb';
import { adapter, makeTransport } from './adapter.ts';

class Server extends Effect.Service<Server>()('Server', {
  scoped: Effect.gen(function* () {
    const path = yield* Path.Path;
    const fs = yield* FileSystem.FileSystem;

    const dist = yield* pipe(
      import.meta.resolve('@the-dev-tools/server'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    );

    const socketPath = path.resolve(os.tmpdir(), 'the-dev-tools', 'test.auth-adapter.server.socket');

    const db = { name: 'state', path: path.resolve(import.meta.dirname, '..') };

    yield* Effect.addFinalizer(() => pipe(path.resolve(db.path, db.name + '.db'), fs.remove, Effect.ignore));

    const process = yield* pipe(
      path.join(dist, 'server'),
      Command.make,
      Command.env({
        DB_ENCRYPTION_KEY: 'secret',
        DB_MODE: 'local',
        DB_NAME: db.name,
        DB_PATH: db.path,
        SERVER_SOCKET_PATH: socketPath,
      }),
      Command.stdout('inherit'),
      Command.stderr('inherit'),
      Command.start,
    );

    // Wait for the server to start up
    yield* pipe(
      Effect.tryPromise((signal) =>
        makeTransport(socketPath).unary(HealthService.method.healthCheck, signal, 0, undefined, {}),
      ),
      Effect.retry({ schedule: Schedule.fixed('0.5 seconds'), times: 30 }),
    );

    return { process, socketPath };
  }),
}) {}

const runtime = pipe(
  Layer.empty,
  Layer.provideMerge(Server.Default),
  Layer.provideMerge(NodeContext.layer),
  ManagedRuntime.make,
);

beforeAll(() => runtime.runPromise(Server));
afterAll(() => runtime.dispose());

describe('Adapter Tests', async () => {
  const { socketPath } = await runtime.runPromise(Server);

  runAdapterTest({
    getAdapter: (_ = {}) => adapter({ debugLogs: { isRunningAdapterTests: true }, socketPath })(_),
    // IDs are stored as 16-byte ULID BLOBs — arbitrary string IDs like "mocked-id" cannot be stored.
    disableTests: { SHOULD_PREFER_GENERATE_ID_IF_PROVIDED: true },
  });
});
