import Prettier from '@prettier/sync';
import { Array, pipe, Record, String } from 'effect';
import { execSync } from 'node:child_process';
import FS from 'node:fs';
import Path from 'node:path';
import { fileURLToPath } from 'node:url';
import packageJson from './package.json';

const modulePath = fileURLToPath(import.meta.url);
const projectPath = Path.dirname(modulePath);
const outputPath = Path.resolve(projectPath, 'dist/typescript');
const packageJsonPath = Path.resolve(projectPath, 'package.json');

FS.rmSync(outputPath, { recursive: true, force: true });

execSync('pnpm buf generate', { cwd: projectPath });

const toExport = (file: string) => {
  const entrypoint = './' + file.slice(0, -3);

  const path = pipe(
    file,
    (_) => Path.join(outputPath, _),
    (_) => Path.relative(projectPath, _),
    (_) => './' + _,
  );

  return [entrypoint, path] as const;
};

const exports = pipe(
  FS.readdirSync(outputPath, { encoding: null, recursive: true }),
  Array.filter(String.endsWith('.ts')),
  Array.map(toExport),
  Record.fromEntries,
);

const packageJsonNew = pipe(
  { ...packageJson, exports },
  (_) => JSON.stringify(_, undefined, 2),
  (_) => Prettier.format(_, { filepath: packageJsonPath }),
);

FS.writeFileSync(packageJsonPath, packageJsonNew);
