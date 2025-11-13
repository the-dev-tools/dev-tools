import { createClient } from '@connectrpc/connect';
import { eq, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import { useContext, useState } from 'react';
import { ReferenceService } from '@the-dev-tools/spec/api/reference/v1/reference_pb';
import { HttpBodyRawCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import {
  baseCodeMirrorExtensions,
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~/code-mirror/extensions';
import { guessLanguage } from '~/guess-language';
import { prettierFormat } from '~/prettier';
import { useReactRender } from '~/react-render';
import { ReferenceContext } from '~/reference';
import { rootRouteApi } from '~/routes';

export interface RawFormProps {
  httpId: Uint8Array;
}

export const RawForm = ({ httpId }: RawFormProps) => {
  const { transport } = rootRouteApi.useRouteContext();

  const collection = useApiCollection(HttpBodyRawCollectionSchema);

  const data = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, httpId))
        .findOne(),
    [collection, httpId],
  ).data?.data;

  const save = (value: string) => {
    if (data === undefined) collection.utils.insert({ data: value, httpId });
    else collection.utils.update({ data: value, httpId });
  };

  const [value, setValue] = useState(data ?? '');
  const [language, setLanguage] = useState<CodeMirrorMarkupLanguage>(guessLanguage(data ?? ''));

  // Get base language extensions
  const languageExtensions = useCodeMirrorLanguageExtensions(language);

  // Get reference context and setup for variable autocompletion
  const context = useContext(ReferenceContext);
  const client = createClient(ReferenceService, transport);
  const reactRender = useReactRender();

  // TODO: use pre-composed extensions instead of duplicating code here
  // Combine language extensions with reference extensions
  const combinedExtensions = [...languageExtensions, ...baseCodeMirrorExtensions({ client, context, reactRender })];

  return (
    <>
      <div className={tw`flex items-center gap-2`}>
        <Select
          aria-label='Language'
          className={tw`self-center justify-self-start`}
          onSelectionChange={(_) => void setLanguage(_ as CodeMirrorMarkupLanguage)}
          selectedKey={language}
          triggerClassName={tw`px-4 py-1`}
        >
          {CodeMirrorMarkupLanguages.map((_) => (
            <SelectItem id={_} key={_}>
              {_}
            </SelectItem>
          ))}
        </Select>

        <Button
          className={tw`px-4 py-1`}
          onPress={async () => {
            const formattedValue = await prettierFormat({ language, text: value });
            setValue(formattedValue);
            save(formattedValue);
          }}
        >
          Prettify
        </Button>
      </div>

      <CodeMirror
        className={tw`col-span-full self-stretch`}
        extensions={combinedExtensions}
        height='100%'
        onBlur={() => void save(value)}
        onChange={setValue}
        value={value}
      />
    </>
  );
};
