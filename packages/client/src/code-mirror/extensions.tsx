import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  Completion,
  completionKeymap,
  CompletionSource,
  ifIn,
} from '@codemirror/autocomplete';
import { history, historyKeymap, standardKeymap } from '@codemirror/commands';
import {
  bracketMatching,
  defaultHighlightStyle,
  LanguageSupport,
  LRLanguage,
  syntaxHighlighting,
} from '@codemirror/language';
import { Extension } from '@codemirror/state';
import { keymap } from '@codemirror/view';
import { Client } from '@connectrpc/connect';
import { styleTags, tags } from '@lezer/highlight';
import { useQuery } from '@tanstack/react-query';
import { Array, Match, pipe } from 'effect';

import { ReferenceKind, ReferenceService } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { ReferenceContextProps } from '~reference';

import { parser } from './syntax.grammar';

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

interface ReferenceCompletionsProps {
  client: Client<typeof ReferenceService>;
  context: ReferenceContextProps;
}

const referenceCompletions =
  ({ client, context: referenceContext }: ReferenceCompletionsProps): CompletionSource =>
  async (context) => {
    const token = context.tokenBefore(['Reference']);

    if (!token) return null;

    const options = pipe(
      (await client.referenceCompletion({ ...referenceContext, start: token.text })).items,
      Array.map((_): Completion => {
        const type = pipe(
          Match.value(_.kind),
          Match.when(ReferenceKind.VALUE, () => 'class'),
          Match.when(ReferenceKind.VARIABLE, () => 'variable'),
          Match.when(ReferenceKind.MAP, () => 'property'),
          Match.when(ReferenceKind.ARRAY, () => 'property'),
          Match.orElse(() => undefined!),
        );

        const detail = pipe(
          Match.value(_),
          Match.when({ kind: ReferenceKind.MAP }, (_) => `${_.itemCount} keys`),
          Match.when({ kind: ReferenceKind.ARRAY }, (_) => `${_.itemCount} entries`),
          Match.orElse(() => undefined!),
        );

        const label = _.endToken.substring(_.endIndex);

        return {
          detail,
          displayLabel: _.endToken,
          label,
          type,
        };
      }),
    );

    return {
      commitCharacters: ['.'],
      filter: false,
      from: token.to,
      getMatch: (_) => {
        if (!_.displayLabel) return [];
        const endIndex = _.displayLabel.length - _.label.length;
        return [0, endIndex];
      },
      options,
    };
  };

interface LanguageProps extends ReferenceCompletionsProps {}

const language = (props: LanguageProps) => {
  const lrl = LRLanguage.define({
    parser: parser.configure({
      props: [
        styleTags({
          CloseMarker: tags.escape,
          OpenMarker: tags.escape,
          Reference: tags.string,
        }),
      ],
    }),
  });

  return new LanguageSupport(lrl, [
    lrl.data.of({
      autocomplete: ifIn(['Reference'], referenceCompletions(props)),
    }),
  ]);
};

const keymaps = keymap.of([...standardKeymap, ...historyKeymap, ...closeBracketsKeymap, ...completionKeymap]);

interface BaseCodeMirrorExtensionProps extends ReferenceCompletionsProps {}

export const baseCodeMirrorExtensions = (props: BaseCodeMirrorExtensionProps): Extension[] => [
  keymaps,
  history(),
  closeBrackets(),
  autocompletion({ activateOnCompletion: () => true, selectOnOpen: false }),
  syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
  bracketMatching(),
  language(props),
];
