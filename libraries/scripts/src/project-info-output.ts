import { execSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';

import { goToRoot } from './go-to-root.ts';

goToRoot();

const tag = process.env['GITHUB_REF_TYPE'] === 'tag' ? process.env['GITHUB_REF_NAME'] : undefined;
const [name, version] = tag?.split('@') ?? [];
if (!name || !version) throw new Error('Could not determine project name or version');

const infoJson = execSync(`nx show project ${name} --json`, { encoding: 'utf8' });
const info: unknown = JSON.parse(infoJson);
if (!info) throw new Error('Could not determine project information');

const output = Object.entries({ ...info, version })
  .map(([key, value]) => {
    if (typeof value !== 'string') return '';

    // $exampleKey -> EXAMPLE_KEY
    const varKey = key
      .replace(/([A-Z])/g, '_$1')
      .toUpperCase()
      .replace(/[^A-Z_]/g, '');

    return `echo "${varKey}=${value}" >> "$GITHUB_OUTPUT"`;
  })
  .join('\n');

writeFileSync('project-info-output', output);
