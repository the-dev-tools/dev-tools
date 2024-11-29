import { pipe } from 'effect';
import { mkdirSync, readdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { join, dirname, basename, parse, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = pipe('../dist/@typespec/protobuf/', import.meta.resolve, fileURLToPath);
const dirents = readdirSync(dir, { recursive: true, withFileTypes: true });

/** @type {[string]} */
const newPaths = [];

/** @type {[[string,string]]}*/
const importChanges = [];

for (const dirent of dirents) {
  if (!dirent.isFile()) continue;
  const oldPath = join(dirent.parentPath, dirent.name);

  const path1 = parse(dirent.name).name;
  const path2 = basename(dirent.parentPath) + '.proto';
  const newPath = join(dirent.parentPath, path1, path2);

  newPaths.push(newPath);

  const oldImport = oldPath.replace(dir, '');
  const newImport = newPath.replace(dir, '');

  importChanges.push([oldImport, newImport]);

  mkdirSync(dirname(newPath), { recursive: true });
  renameSync(oldPath, newPath);
}

for (const path of newPaths) {
  let file = readFileSync(path, { encoding: 'utf-8' });
  for (const [oldImport, newImport] of importChanges) {
    file = file.replace(oldImport, newImport);
  }
  writeFileSync(path, file, { encoding: 'utf-8' });
}
