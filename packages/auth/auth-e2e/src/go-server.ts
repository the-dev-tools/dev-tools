import { spawn } from 'node:child_process';
import path from 'node:path';

// Resolve the packages/server directory relative to this source file.
// __dirname in ESM is available via import.meta.dirname (Node ≥20).
const SERVER_DIR = path.resolve(import.meta.dirname, '../../../server');

export interface GoServer {
  socketPath: string;
  kill(): void;
}

/**
 * Spawns the authadapter-testserver Go binary and waits until it prints
 * "READY" to stdout before resolving.
 *
 * The server listens on a unique Unix socket path and exits on SIGTERM.
 */
export async function startGoServer(): Promise<GoServer> {
  const socketPath = `/tmp/authadapter-e2e-${process.pid.toString()}-${Date.now().toString()}.socket`;

  const proc = spawn('go', ['run', './cmd/authadapter-testserver'], {
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
