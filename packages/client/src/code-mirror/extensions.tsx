import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  Completion,
  completionKeymap,
  CompletionSource,
  pickedCompletion,
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
import { ChangeSpec, EditorSelection, EditorState, Extension, Prec, Text } from '@codemirror/state';
import { EditorView, keymap, tooltips } from '@codemirror/view';
import { Client } from '@connectrpc/connect';
import { styleTags, tags } from '@lezer/highlight';
import { useQuery } from '@tanstack/react-query';
import { Array, Match, pipe } from 'effect';
import { Suspense } from 'react';
import { LuClipboardCopy } from 'react-icons/lu';
import {
  ReferenceCompletion,
  ReferenceKind,
  ReferenceService,
} from '@the-dev-tools/spec/buf/api/reference/v1/reference_pb';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectSuspenseQuery } from '~/api/connect-query';
import { ReactRender } from '~/react-render';
import { ReferenceContextProps } from '~/reference';
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
  } = useConnectSuspenseQuery(ReferenceService.method.referenceValue, { ...context, path });

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
  allowFiles?: boolean | undefined;
  client: Client<typeof ReferenceService>;
  context: ReferenceContextProps;
  reactRender: ReactRender;
}

// TODO: fix implementation
const referenceCompletions =
  ({
    allowFiles = false,
    client,
    context: referenceContext,
    reactRender,
  }: ReferenceCompletionsProps): CompletionSource =>
  async (context) => {
    // Check for Reference token type first (works in text body)
    let token = context.tokenBefore(['Word'])?.text.trimStart();

    const isExpression =
      context.tokenBefore(['String', 'StringExpression']) === null || context.tokenBefore(['Interpolation']) !== null;

    if (token === undefined && isExpression) token = '';

    // If no Reference token found, check if we have JSON string content with variables
    if (token === undefined) {
      // Look for JSON string tokens that might contain variable references
      const line = context.state.doc.lineAt(context.pos);
      const lineText = line.text;

      // Find '{{' pattern in the current line before the cursor position
      const cursorPosInLine = context.pos - line.from;
      const beforeCursor = lineText.substring(0, cursorPosInLine);
      const openBraceIndex = beforeCursor.lastIndexOf('{{');

      if (openBraceIndex >= 0) {
        // Extract potential variable reference text
        token = beforeCursor.substring(openBraceIndex + 2).trim();
      }
    }

    // Special handling for JSON string context
    // Look for tokens of different types that might be inside a JSON string
    if (token === undefined) {
      // Get the token at the current position
      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any
      const tree = (context.state as any).syntaxTree;
      // Add null check to prevent TypeError
      if (!tree) return null;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      const tokenAtCursor = tree.resolveInner(context.pos);

      // If we're in a string token (JSON or otherwise)
      // eslint-disable-next-line @typescript-eslint/no-unsafe-argument, @typescript-eslint/no-unsafe-member-access
      if (tokenAtCursor && /string/i.test(tokenAtCursor.type.name)) {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-argument, @typescript-eslint/no-unsafe-member-access
        const stringContent = context.state.doc.sliceString(tokenAtCursor.from, tokenAtCursor.to);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
        const cursorOffsetInString = context.pos - tokenAtCursor.from;
        const textBeforeCursor = stringContent.substring(0, cursorOffsetInString);

        // Check if there's a '{{' before the cursor in this string
        const varStartIndex = textBeforeCursor.lastIndexOf('{{');

        if (varStartIndex >= 0) {
          // Extract the variable reference text
          token = textBeforeCursor.substring(varStartIndex + 2);
        }
      }
    }

    if (token === undefined) return null;

    let options: Completion[] = [];

    const fileToken = '#file:';
    if (allowFiles && fileToken.startsWith(token)) {
      options.push({
        apply: async (view, completion, from) => {
          const { filePaths } = await window.electron.dialog('showOpenDialog', {});
          const path = filePaths[0];
          if (!path) return;

          const insert = completion.label + path;

          view.dispatch({
            annotations: pickedCompletion.of(completion),
            changes: [{ from, insert }],
            selection: { anchor: from + insert.length },
          });
        },
        displayLabel: fileToken,
        label: fileToken.replace(token, ''),
      });
    }

    options = pipe(
      (await client.referenceCompletion({ ...referenceContext, start: token })).items,
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
        const path = token + label;

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
      Array.appendAll(options),
    );

    return {
      commitCharacters: ['.'],
      filter: false,
      from: context.pos,
      getMatch: (_) => {
        if (!_.displayLabel) return [];
        const endIndex = _.displayLabel.length - _.label.length;
        return [0, endIndex];
      },
      options,
    };
  };

interface LanguageProps extends ReferenceCompletionsProps {
  kind?: 'FullExpression' | 'StringExpression' | undefined;
}

const language = ({ kind = 'FullExpression', ...props }: LanguageProps) => {
  const lrl = LRLanguage.define({
    parser: parser.configure({
      top: kind,

      props: [
        styleTags({
          InterpolationEnd: tags.escape,
          InterpolationStart: tags.escape,
          String: tags.string,
          StringExpression: tags.string,
        }),
      ],
    }),
  });

  return new LanguageSupport(lrl, [
    lrl.data.of({
      // Handle both direct Reference tokens and other contexts
      autocomplete: referenceCompletions(props),
    }),
  ]);
};

const expressionBracketSpacing = EditorView.updateListener.of((update) => {
  if (update.changes.empty) return;

  // {{|}} --> {{ | }}
  update.changes.iterChanges((_fromA, _toA, fromB, toB, inserted) => {
    const doc = update.state.doc;

    // Handle the typical variable template insertion
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

    // Handle when a user types '{{' in JSON or other content
    // This will trigger autocompletion after typing '{{'
    if (inserted.eq(Text.of(['{'])) && doc.sliceString(fromB - 1, fromB) === '{') {
      startCompletion(update.view);
    }
  });
});

// https://discuss.codemirror.net/t/codemirror-6-single-line-and-or-avoid-carriage-return/2979/8
const singleLineModeExtensions = [
  EditorState.transactionFilter.of((tr) => {
    if (tr.changes.empty) return tr;
    if (tr.newDoc.lines > 1 && !tr.isUserEvent('input.paste')) {
      return [];
    }

    const removeNLs: ChangeSpec[] = [];
    tr.changes.iterChanges((_fromA, _toA, fromB, _toB, ins) => {
      const lineIter = ins.iterLines().next();
      if (ins.lines <= 1) return;
      // skip the first line
      let len = fromB + lineIter.value.length;
      lineIter.next();
      // for the next lines, remove the leading NL
      for (; !lineIter.done; lineIter.next()) {
        removeNLs.push({ from: len, to: len + 1 });
        len += lineIter.value.length + 1;
      }
    });

    return [tr, { changes: removeNLs, sequential: true }];
  }),

  Prec.high(
    keymap.of([
      { key: 'ArrowUp', run: () => true },
      { key: 'ArrowDown', run: () => true },
    ]),
  ),
];

const keymaps = keymap.of([...standardKeymap, ...historyKeymap, ...closeBracketsKeymap, ...completionKeymap]);

export interface BaseCodeMirrorExtensionProps extends LanguageProps {
  singleLineMode?: boolean;
}

// Additional handler to trigger completions in JSON strings
const jsonStringCompletionHandler = EditorView.updateListener.of((update) => {
  if (!update.docChanged) return;

  // Look for typing "{{" in the current document
  const pos = update.state.selection.main.head;
  const line = update.state.doc.lineAt(pos);
  const lineText = line.text;

  // Check if the cursor is after a "{{" pattern in the current line
  const cursorPosInLine = pos - line.from;
  const beforeCursor = lineText.substring(0, cursorPosInLine);

  // Trigger completion in two scenarios:
  // 1. After typing '{{' anywhere
  if (beforeCursor.endsWith('{{')) {
    startCompletion(update.view);
    return;
  }

  // 2. When inside a JSON string that contains '{{'
  const openBraceIndex = beforeCursor.lastIndexOf('{{');
  if (openBraceIndex >= 0) {
    // In a potential JSON string context if there's a quote before the {{
    // and the {{ appears after the last quote
    const lastQuoteIndex = beforeCursor.lastIndexOf('"');
    if (
      lastQuoteIndex < openBraceIndex &&
      // Make sure we're still inside the string (check for " after cursor)
      lineText.includes('"', cursorPosInLine)
    ) {
      startCompletion(update.view);
    }
  }
});

export const baseCodeMirrorExtensions = ({ singleLineMode, ...props }: BaseCodeMirrorExtensionProps): Extension[] => {
  const extensions = [
    tooltips({
      parent: document.getElementById('cm-label-layer')!,
      position: 'fixed',
    }),
    keymaps,
    history(),
    closeBrackets(),
    autocompletion({
      activateOnCompletion: () => true,
      override: [referenceCompletions(props)],
      selectOnOpen: false,
    }),
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    expressionBracketSpacing,
    jsonStringCompletionHandler,
    bracketMatching(),
    language(props),
  ];

  if (singleLineMode) extensions.push(...singleLineModeExtensions);

  return extensions;
};
