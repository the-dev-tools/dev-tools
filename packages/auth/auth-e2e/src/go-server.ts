import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const SERVER_DIR = path.resolve(import.meta.dirname, '../../../server');

// Prefer the pre-built binary produced by `nx run server:build-testserver`.
// Fall back to `go run` for developers who haven't run the build target.
const PREBUILT = path.join(SERVER_DIR, 'authadapter-testserver');
const usePrebuilt = fs.existsSync(PREBUILT);

const cmd = usePrebuilt ? PREBUILT : 'go';
const args = usePrebuilt ? [] : ['run', './cmd/authadapter-testserver'];

export interface GoServer {
  socketPath: string;
  kill(): void;
}

/**
 * Spawns the authadapter-testserver Go binary and waits until it prints
 * "READY" to stdout before resolving.
 *
 * Prefers a pre-built binary at packages/server/authadapter-testserver
 * (produced by `nx run server:build-testserver`). Falls back to `go run`
 * when the binary is absent (local dev without the build step).
 */
export async function startGoServer(): Promise<GoServer> {
  const socketPath = `/tmp/authadapter-e2e-${process.pid.toString()}-${Date.now().toString()}.socket`;

  const proc = spawn(cmd, args, {
    cwd: SERVER_DIR,
    env: { ...process.env, SOCKET_PATH: socketPath },
    stdio: ['ignore', 'pipe', 'inherit'],
  });

  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error('Go server timed out waiting for READY signal'));
    }, 30_000);

    proc.stdout.on('data', (chunk: Buffer) => {
      if (chunk.toString().includes('READY')) {
        clearTimeout(timeout);
        resolve();
      }
    });

    proc.on('error', (err) => {
      clearTimeout(timeout);
      reject(err);
    });

    proc.on('exit', (code) => {
      if (code !== null) {
        clearTimeout(timeout);
        reject(new Error(`Go server exited with code ${code.toString()}`));
      }
    });
  });

  return {
    socketPath,
    kill: () => {
      proc.kill('SIGTERM');
    },
  };
}
