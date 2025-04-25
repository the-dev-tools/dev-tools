import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  Completion,
  completionKeymap,
  CompletionSource,
  ifIn,
  startCompletion,
} from '@codemirror/autocomplete';
import { history, historyKeymap, standardKeymap } from '@codemirror/commands';
import {
  bracketMatching,
  defaultHighlightStyle,
  LanguageSupport,
  LRLanguage,
  syntaxHighlighting,
} from '@codemirror/language';
import { EditorSelection, Extension, Text } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import { Client } from '@connectrpc/connect';
import { styleTags, tags } from '@lezer/highlight';
import { useQuery } from '@tanstack/react-query';
import { Array, Match, pipe } from 'effect';
import { Suspense } from 'react';
import { LuClipboardCopy } from 'react-icons/lu';

import { ReferenceCompletion, ReferenceKind, ReferenceService } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { referenceValue } from '@the-dev-tools/spec/reference/v1/reference-ReferenceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectSuspenseQuery } from '~api/connect-query';
import { ReactRender } from '~react-render';
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

interface CompletionInfoProps {
  completion: ReferenceCompletion;
  context: ReferenceContextProps;
  path: string;
}

const CompletionInfo = ({ completion, context, path }: CompletionInfoProps) => {
  const {
    data: { value },
  } = useConnectSuspenseQuery(referenceValue, { ...context, path });

  return (
    <>
      <div className={tw`flex items-center gap-1`}>
        <div className={tw`font-semibold`}>Value:</div>

        <div>{value}</div>

        <Button
          className={tw`p-0.5`}
          onClick={async () => {
            await navigator.permissions.query({ name: 'clipboard-write' as never });
            await navigator.clipboard.writeText(value);
          }}
          variant='ghost'
        >
          <LuClipboardCopy className={tw`size-4 text-slate-500`} />
        </Button>
      </div>

      {completion.kind === ReferenceKind.VARIABLE && (
        <div>
          <div className={tw`font-semibold`}>Variable defined in environments:</div>
          <ul>
            {completion.environments.map((name, index) => (
              <li key={`${index} ${name}`}>{name}</li>
            ))}
          </ul>
        </div>
      )}
    </>
  );
};

interface ReferenceCompletionsProps {
  client: Client<typeof ReferenceService>;
  context: ReferenceContextProps;
  reactRender: ReactRender;
}

const referenceCompletions =
  ({ client, context: referenceContext, reactRender }: ReferenceCompletionsProps): CompletionSource =>
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
        const path = token.text + label;

        const info = () => {
          if (![ReferenceKind.VALUE, ReferenceKind.VARIABLE].includes(_.kind)) return null;

          return reactRender(
            <div className={tw`w-60 text-sm`}>
              <Suspense fallback='Loading...'>
                <CompletionInfo completion={_} context={referenceContext} path={path} />
              </Suspense>
            </div>,
          );
        };

        return {
          detail,
          displayLabel: _.endToken,
          info,
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

const expressionBracketSpacing = EditorView.updateListener.of((update) => {
  if (update.changes.empty) return;

  // {{|}} --> {{ | }}
  update.changes.iterChanges((_fromA, _toA, fromB, toB, inserted) => {
    const doc = update.state.doc;
    if (
      inserted.eq(Text.of(['{}'])) &&
      doc.sliceString(fromB - 1, fromB) === '{' &&
      doc.sliceString(toB, toB + 1) === '}'
    ) {
      update.view.dispatch({
        changes: [{ from: fromB + 1, insert: '  ' }],
        selection: EditorSelection.cursor(toB),
      });
      startCompletion(update.view);
    }
  });
});

const keymaps = keymap.of([...standardKeymap, ...historyKeymap, ...closeBracketsKeymap, ...completionKeymap]);

interface BaseCodeMirrorExtensionProps extends ReferenceCompletionsProps {}

export const baseCodeMirrorExtensions = (props: BaseCodeMirrorExtensionProps): Extension[] => [
  keymaps,
  history(),
  closeBrackets(),
  autocompletion({ activateOnCompletion: () => true, selectOnOpen: false }),
  syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
  expressionBracketSpacing,
  bracketMatching(),
  language(props),
];
