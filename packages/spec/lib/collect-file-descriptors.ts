import { pipe } from 'effect';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = pipe('../buf/typescript/', (_) => import.meta.resolve(_), fileURLToPath);
const dirents = readdirSync(dir, { recursive: true, withFileTypes: true });

const imports = [];

const exports = [];

for (const dirent of dirents) {
  if (!dirent.isFile()) continue;
  if (!dirent.name.endsWith('_pb.ts')) continue;

  const file = path.join(dirent.parentPath, dirent.name);
  const data = readFileSync(file, { encoding: 'utf-8' });

  const importPath = file.replace(dir, './').replaceAll(path.sep, path.posix.sep);
  const exportName = /(?<=export const )file_.*(?=: GenFile)/.exec(data)?.[0];

  if (exportName === undefined) continue;

  imports.push(`import { ${exportName} } from '${importPath}';`);
  exports.push(`  ${exportName},`);
}

const content = `
${imports.join('\n')}

export const files = [
${exports.join('\n')}
];
`;

writeFileSync(path.join(dir, 'files.ts'), content, { encoding: 'utf-8' });
