import { execSync } from 'node:child_process';
import { appendFileSync } from 'node:fs';

const tag = process.env['GITHUB_REF_TYPE'] === 'tag' ? process.env['GITHUB_REF_NAME'] : undefined;
const [name, version] = tag?.split('@') ?? [];
if (!name || !version) throw new Error('Could not determine project name or version');

const infoJson = execSync(`nx show project ${name} --json`, { encoding: 'utf8' });
const info: unknown = JSON.parse(infoJson);
if (!info) throw new Error('Could not determine project information');

Object.entries({ ...info, version }).forEach(([key, value]) => {
  if (typeof value !== 'string') return;

  // $exampleKey -> EXAMPLE_KEY
  const varKey = key
    .replace(/([A-Z])/g, '_$1')
    .toUpperCase()
    .replace(/[^A-Z_]/g, '');

  appendFileSync(process.env['GITHUB_OUTPUT'] ?? '', `${varKey}=${value}\n`);
});
