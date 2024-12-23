import * as FS from 'node:fs';
import * as Path from 'node:path';
import { Array, flow, identity, Match, Number, Option, pipe, Record, String, Struct, Tuple } from 'effect';
import * as Tailwind from 'tailwindcss';
import resolveConfig from 'tailwindcss/resolveConfig';

import { keyValue } from '@the-dev-tools/utils/helpers';

import rawConfig from './config';

interface TokenRecord<T> extends Record<string, { type: T; value: string } | TokenRecord<T>> {}

interface TailwindRecord extends Record.ReadonlyRecord<string, TailwindRecord | string> {}

const mapKey = (key: string) => pipe(key, String.replaceAll('.', ','), String.replaceAll('/', '\\'));

const mapTokenEntry =
  <T>(type: T) =>
  (value: TailwindRecord | string, key: string): [string, TokenRecord<T>[string]] => {
    if (typeof value === 'string') return [mapKey(key), { type, value }];
    return [mapKey(key), Record.mapEntries(value, mapTokenEntry(type))];
  };

const mapTokenType = <T extends string>(type: T, record: TailwindRecord) =>
  keyValue(type, Record.mapEntries(record, mapTokenEntry(type)));

const toPercentage = (self: string) =>
  pipe(self, parseFloat, Number.multiply(100), (_) => _.toString(), String.concat('%'));

const config = resolveConfig(rawConfig as Tailwind.Config);

const boxShadows = pipe(
  config.theme.boxShadow,
  Struct.omit('none'),
  Record.mapKeys(mapKey),
  Record.map(
    flow(
      String.split(', '),
      Array.map(
        flow(
          String.match(/[^\s(]+(\(.+\))?/g), // https://stackoverflow.com/a/62648896
          Option.map((_) => {
            const type = _[0] === 'inset' ? 'innerShadow' : 'dropShadow';
            if (type === 'innerShadow') _.shift();
            const color = pipe(_[4] ?? '', String.replaceAll(' / ', ','), String.replaceAll(' ', ','));
            return { type, x: _[0], y: _[1], blur: _[2], spread: _[3], color };
          }),
        ),
      ),
      Array.getSomes,
      (_) => ({ type: 'boxShadow', value: _ }),
    ),
  ),
);

const core = {
  ...mapTokenType('color', { ...config.theme.colors }),
  ...mapTokenType('spacing', { ...config.theme.spacing }),
  ...mapTokenType('sizing', { ...config.theme.size }),
  ...mapTokenType('borderRadius', { ...config.theme.borderRadius }),
  ...mapTokenType('borderWidth', { ...config.theme.borderWidth }),
  boxShadows,
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

// Typography presets
const tFontSizes = ['xs', 'sm', 'base', 'lg', 'xl'];
const tFontWeights = ['light', 'normal', 'medium'];
const tLineHeights = ['none', 'tight', 'normal'];

const typography = pipe(
  Array.cartesian(tFontSizes, tFontWeights),
  Array.cartesianWith(tLineHeights, ([a, b], c) => [a, b, c] as const),
  Array.map(
    ([fontSize, fontWeight, lineHeight]) =>
      [
        `${fontSize}-${fontWeight}-${lineHeight}`,
        {
          type: 'typography',
          value: {
            fontSize: `{fontSizes.${fontSize}}`,
            fontWeight: `{fontWeights.${fontWeight}}`,
            lineHeight: `{lineHeights.${lineHeight}}`,
            fontFamily: '{fontFamilies.sans}',
            letterSpacing: '{letterSpacing.normal}',
          },
        },
      ] as const,
  ),
  Record.fromEntries,
);

const dist = Path.resolve(__dirname, '../dist/design-tokens');
FS.mkdirSync(dist, { recursive: true });
FS.writeFileSync(Path.resolve(dist, 'core.json'), JSON.stringify(core, null, 2));
FS.writeFileSync(Path.resolve(dist, 'typography.json'), JSON.stringify(typography, null, 2));
