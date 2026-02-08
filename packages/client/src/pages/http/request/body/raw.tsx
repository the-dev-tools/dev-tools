import { createClient } from '@connectrpc/connect';
import CodeMirror from '@uiw/react-codemirror';
import { useContext, useState } from 'react';
import { ReferenceService } from '@the-dev-tools/spec/buf/api/reference/v1/reference_pb';
import {
  HttpBodyRawCollectionSchema,
  HttpBodyRawDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { useTheme } from '@the-dev-tools/ui';
import { Button } from '@the-dev-tools/ui/button';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { DeltaResetButton, useDeltaState } from '~/features/delta';
import {
  baseCodeMirrorExtensions,
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  guessLanguage,
  prettierFormat,
  ReferenceContext,
  useCodeMirrorLanguageExtensions,
} from '~/features/expression';
import { useReactRender } from '~/shared/lib';
import { routes } from '~/shared/routes';

export interface RawFormProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const RawForm = ({ deltaHttpId, httpId, isReadOnly = false }: RawFormProps) => {
  const { transport } = routes.root.useRouteContext();

  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpBodyRawDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpBodyRawCollectionSchema,
    valueKey: 'data',
  } as const;

  const [value, setValue] = useDeltaState(deltaOptions);

  const [language, setLanguage] = useState<CodeMirrorMarkupLanguage>(guessLanguage(value ?? ''));

  // Get base language extensions
  const languageExtensions = useCodeMirrorLanguageExtensions(language);

  // Get reference context and setup for variable autocompletion
  const context = useContext(ReferenceContext);
  const client = createClient(ReferenceService, transport);
  const reactRender = useReactRender();

  const { resolvedTheme } = useTheme();

  // TODO: use pre-composed extensions instead of duplicating code here
  // Combine language extensions with reference extensions
  const combinedExtensions = [...languageExtensions, ...baseCodeMirrorExtensions({ client, context, reactRender })];

  return (
    <>
      <div className={tw`flex items-center gap-2`}>
        <Select
          aria-label='Language'
          className={tw`self-center justify-self-start`}
          onChange={(_) => void setLanguage(_ as CodeMirrorMarkupLanguage)}
          triggerClassName={tw`px-4 py-1`}
          value={language}
        >
          {CodeMirrorMarkupLanguages.map((_) => (
            <SelectItem id={_} key={_}>
              {_}
            </SelectItem>
          ))}
        </Select>

        {!isReadOnly && (
          <Button
            className={tw`px-4 py-1`}
            onPress={async () => {
              const formatted = await prettierFormat({ language, text: value ?? '' });
              setValue(formatted);
            }}
          >
            Prettify
          </Button>
        )}

        {!isReadOnly && <DeltaResetButton {...deltaOptions} />}
      </div>

      <CodeMirror
        className={tw`col-span-full self-stretch`}
        extensions={combinedExtensions}
        height='100%'
        onChange={(_) => void setValue(_)}
        readOnly={isReadOnly}
        theme={resolvedTheme}
        value={value ?? ''}
      />
    </>
  );
};
