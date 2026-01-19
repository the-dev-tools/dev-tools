import { queryOptions } from '@tanstack/react-query';
import { Array, Match, pipe } from 'effect';
import { format } from 'prettier/standalone';
import { CodeMirrorMarkupLanguage } from './code-mirror/extensions';

export interface PrettierFormatProps {
  language: CodeMirrorMarkupLanguage;
  text: string;
}

export const prettierFormat = async ({ language, text }: PrettierFormatProps) => {
  if (language === 'text') return text;

  const plugins = await pipe(
    Match.value(language),
    Match.when('json', () => [import('prettier/plugins/estree'), import('prettier/plugins/babel')]),
    Match.when('html', () => [import('prettier/plugins/html')]),
    Match.when('xml', () => [import('@prettier/plugin-xml')]),
    Match.exhaustive,
    Array.map((_) => _.then((_) => _.default)),
    (_) => Promise.all(_),
  );

  const parser = pipe(
    Match.value(language),
    Match.when('json', () => 'json-stringify'),
    Match.orElse((_) => _),
  );

  return await format(text, {
    htmlWhitespaceSensitivity: 'ignore',
    parser,
    plugins,
    printWidth: 100,
    singleAttributePerLine: true,
    tabWidth: 2,
    xmlWhitespaceSensitivity: 'ignore',
  }).catch(() => text);
};

export const prettierFormatQueryOptions = (props: PrettierFormatProps) =>
  queryOptions({
    initialData: 'Formatting...',
    queryFn: () => prettierFormat(props),
    queryKey: ['prettier', props],
  });
