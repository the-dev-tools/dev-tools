import { mkdirSync, readdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, parse, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { pipe } from 'effect';

const dir = pipe('../dist/buf/typescript/', import.meta.resolve, fileURLToPath);
const dirents = readdirSync(dir, { recursive: true, withFileTypes: true });

/** @type {string[]}*/
const imports = [];

/** @type {string[]}*/
const exports = [];

for (const dirent of dirents) {
  if (!dirent.isFile()) continue;
  if (!dirent.name.endsWith('_pb.ts')) continue;

  const path = join(dirent.parentPath, dirent.name);
  const file = readFileSync(path, { encoding: 'utf-8' });

  const importPath = path.replace(dir, './');
  const exportName = file.match(/(?<=export const )file_.*(?=: GenFile)/)?.[0];

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

writeFileSync(join(dir, 'files.ts'), content, { encoding: 'utf-8' });
