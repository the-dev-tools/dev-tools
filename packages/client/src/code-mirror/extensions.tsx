import { Extension } from '@codemirror/state';
import { useQuery } from '@tanstack/react-query';
import { Array, Match, pipe } from 'effect';

export const CodeMirrorMarkupLanguages = ['text', 'json', 'html', 'xml'] as const;
export type CodeMirrorMarkupLanguage = (typeof CodeMirrorMarkupLanguages)[number];

export const CodeMirrorLanguages = [...CodeMirrorMarkupLanguages, 'javascript'] as const;
export type CodeMirrorLanguage = (typeof CodeMirrorLanguages)[number];

export const useCodeMirrorLanguageExtensions = (language: CodeMirrorLanguage): Extension[] => {
  const { data: extensions } = useQuery({
    initialData: [],
    queryFn: async () => {
      if (language === 'text') return [];
      return await pipe(
        Match.value(language),
        Match.when('html', () => import('@codemirror/lang-html').then((_) => _.html())),
        Match.when('javascript', () => import('@codemirror/lang-javascript').then((_) => _.javascript())),
        Match.when('json', () => import('@codemirror/lang-json').then((_) => _.json())),
        Match.when('xml', () => import('@codemirror/lang-xml').then((_) => _.xml())),
        Match.exhaustive,
        (_) => _.then(Array.make),
      );
    },
    queryKey: ['code-mirror', language],
  });

  return extensions;
};
