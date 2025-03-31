import { mkdirSync, readdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { pipe } from 'effect';

const dir = pipe('../dist/@typespec/protobuf/', import.meta.resolve, fileURLToPath);
const dirents = readdirSync(dir, { recursive: true, withFileTypes: true });

/** @type {[string]} */
const newPaths = [];

/** @type {[[string,string]]}*/
const importChanges = [];

for (const dirent of dirents) {
  if (!dirent.isFile()) continue;
  const oldPath = path.join(dirent.parentPath, dirent.name);

  const version = path.parse(dirent.name).name;
  const filename = path.basename(dirent.parentPath) + '.proto';
  const newPath = path.join(dirent.parentPath, version, filename);

  newPaths.push(newPath);

  const oldImport = oldPath.replace(dir, '').replaceAll(path.sep, path.posix.sep);
  const newImport = newPath.replace(dir, '').replaceAll(path.sep, path.posix.sep);

  importChanges.push([oldImport, newImport]);

  mkdirSync(path.dirname(newPath), { recursive: true });
  renameSync(oldPath, newPath);
}

for (const filePath of newPaths) {
  let file = readFileSync(filePath, { encoding: 'utf-8' });
  for (const [oldImport, newImport] of importChanges) {
    file = file.replace(oldImport, newImport);
  }
  writeFileSync(filePath, file, { encoding: 'utf-8' });
}
