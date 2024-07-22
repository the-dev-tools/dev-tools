import * as FS from 'node:fs';
import * as Path from 'node:path';
import { Array, flow, identity, Match, Number, pipe, Record, String, Tuple } from 'effect';
import * as Tailwind from 'tailwindcss';
import resolveConfig from 'tailwindcss/resolveConfig';

import { config as rawConfig } from './config';

interface TokenRecord extends Record<string, { value: string } | TokenRecord> {}

interface TailwindRecord extends Record.ReadonlyRecord<string, TailwindRecord | string> {}

const mapKey = (key: string) => pipe(key, String.replaceAll('.', ','), String.replaceAll('/', '\\'));

const mapTokenEntry = (value: TailwindRecord | string, key: string): [string, TokenRecord[string]] => {
  if (typeof value === 'string') return [mapKey(key), { value }];
  return [mapKey(key), Record.mapEntries(value, mapTokenEntry)];
};

const mapTokenType = (type: string, record: TailwindRecord) => ({
  [type]: { type, ...Record.mapEntries(record, mapTokenEntry) },
});

const toPercentage = (self: string) =>
  pipe(self, parseFloat, Number.multiply(100), (_) => _.toString(), String.concat('%'));

const config = resolveConfig(rawConfig as Tailwind.Config);

const tokens = {
  ...mapTokenType('color', { ...config.theme.colors }),
  ...mapTokenType('spacing', { ...config.theme.spacing }),
  ...mapTokenType('sizing', { ...config.theme.size }),
  ...mapTokenType('borderRadius', { ...config.theme.borderRadius }),
  ...mapTokenType('borderWidth', { ...config.theme.borderWidth }),
  ...mapTokenType('opacity', { ...config.theme.opacity }),
  ...mapTokenType('fontFamilies', Record.map(config.theme.fontFamily, Array.join(','))),
  ...mapTokenType('fontWeights', { ...config.theme.fontWeight }),
  ...mapTokenType('fontSizes', Record.map(config.theme.fontSize, Tuple.getFirst)),
  ...mapTokenType(
    'lineHeights',
    Record.map(
      config.theme.lineHeight,
      flow(Match.value, Match.when(String.endsWith('rem'), identity<string>), Match.orElse(toPercentage)),
    ),
  ),
  ...mapTokenType('letterSpacing', Record.map(config.theme.letterSpacing, toPercentage)),
  // https://tailwindcss.com/docs/text-transform
  ...mapTokenType('textCase', {
    uppercase: 'uppercase',
    lowercase: 'lowercase',
    capitalize: 'capitalize',
    'normal-case': 'none',
  }),
  // https://tailwindcss.com/docs/text-decoration
  ...mapTokenType('textDecoration', {
    underline: 'underline',
    overline: 'overline',
    'line-through': 'line-through',
    'no-underline': 'none',
  }),
};

const dist = Path.resolve(__dirname, '../dist');
FS.mkdirSync(dist, { recursive: true });
FS.writeFileSync(Path.resolve(dist, 'tokens.json'), JSON.stringify(tokens, null, 2));
