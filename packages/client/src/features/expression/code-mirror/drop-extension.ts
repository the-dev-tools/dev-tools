import { fromJson } from '@bufbuild/protobuf';
import { StateEffect, StateField } from '@codemirror/state';
import { EditorView, type Extension } from '@codemirror/view';
import { ReferenceKeyJson, ReferenceKeySchema } from '@the-dev-tools/spec/buf/api/reference/v1/reference_pb';
import { referenceKeysToExpression, referenceKeysToJsExpression } from '../reference-path';

export type DropFormat = 'full-expression' | 'javascript' | 'string-expression';

const MIME = 'application/x-devtools-reference';

const setDragOver = StateEffect.define<boolean>();

export const referenceDropExtension = (format: DropFormat): Extension => {
  const dragOver = StateField.define<boolean>({
    create: () => false,
    update: (value, tr) => {
      for (const effect of tr.effects) {
        if (effect.is(setDragOver)) return effect.value;
      }
      return value;
    },
  });

  return [
    dragOver,
    EditorView.domEventHandlers({
      dragenter(event, view) {
        if (event.dataTransfer?.types.includes(MIME)) {
          view.dispatch({ effects: setDragOver.of(true) });
        }
        return false;
      },
      dragleave(event, view) {
        if (!view.dom.contains(event.relatedTarget as Node)) {
          view.dispatch({ effects: setDragOver.of(false) });
        }
        return false;
      },
      dragover(event) {
        if (event.dataTransfer?.types.includes(MIME)) {
          event.preventDefault();
          event.dataTransfer.dropEffect = 'copy';
        }
        return false;
      },
      drop(event, view) {
        view.dispatch({ effects: setDragOver.of(false) });
        const data = event.dataTransfer?.getData(MIME);
        if (!data || view.state.readOnly) return false;

        event.preventDefault();
        const keysJson = JSON.parse(data) as ReferenceKeyJson[];
        const keys = keysJson.map((_) => fromJson(ReferenceKeySchema, _));

        const insertText =
          format === 'javascript'
            ? referenceKeysToJsExpression(keys)
            : referenceKeysToExpression(keys, format === 'string-expression' ? 'StringExpression' : 'FullExpression');

        const pos = view.posAtCoords({ x: event.clientX, y: event.clientY }) ?? view.state.selection.main.head;

        view.dispatch({
          changes: [{ from: pos, insert: insertText }],
          selection: { anchor: pos + insertText.length },
        });
        view.focus();
        return true;
      },
    }),
    EditorView.baseTheme({
      '&.cm-editor.cm-drop-target': { outline: '2px solid var(--color-accent)' },
    }),
    EditorView.updateListener.of((update) => {
      const isDragOver = update.state.field(dragOver);
      update.view.dom.classList.toggle('cm-drop-target', isDragOver);
    }),
  ];
};
