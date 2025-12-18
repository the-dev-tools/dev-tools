import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useQuery } from '@tanstack/react-query';
import CodeMirror from '@uiw/react-codemirror';
import { useState } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { HttpResponseSchema } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { HttpResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import {
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~/code-mirror/extensions';
import { guessLanguage } from '~/guess-language';
import { prettierFormatQueryOptions } from '~/prettier';
import { pick } from '~/utils/tanstack-db';

export interface BodyPanelProps {
  httpResponseId: Uint8Array;
}

export const BodyPanel = ({ httpResponseId }: BodyPanelProps) => {
  const collection = useApiCollection(HttpResponseCollectionSchema);

  const { body } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.httpResponseId, httpResponseId))
          .select((_) => pick(_.item, 'body'))
          .findOne(),
      [collection, httpResponseId],
    ).data ?? create(HttpResponseSchema);

  return (
    <Tabs
      className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'
      defaultSelectedKey='pretty'
    >
      <TabList className='flex gap-1 self-start rounded-md border border-slate-100 bg-slate-100 p-0.5 text-xs leading-5 tracking-tight'>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='pretty'
        >
          Pretty
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='raw'
        >
          Raw
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='preview'
        >
          Preview
        </Tab>
      </TabList>

      <TabPanel className='contents' id='pretty'>
        <BodyPretty body={body} />
      </TabPanel>

      <TabPanel className={tw`col-span-full overflow-auto font-mono whitespace-pre select-text`} id='raw'>
        {body}
      </TabPanel>

      <TabPanel className='col-span-full self-stretch' id='preview'>
        <iframe className='size-full' srcDoc={body} title='Response preview' />
      </TabPanel>
    </Tabs>
  );
};

interface BodyPrettyProps {
  body: string;
}

const BodyPretty = ({ body }: BodyPrettyProps) => {
  const [language, setLanguage] = useState(guessLanguage(body));
  const { data: prettierBody } = useQuery(prettierFormatQueryOptions({ language, text: body }));
  const extensions = useCodeMirrorLanguageExtensions(language);

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
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

      <CodeMirror
        className={tw`col-span-full self-stretch`}
        extensions={extensions}
        height='100%'
        indentWithTab={false}
        readOnly
        value={prettierBody}
      />
    </>
  );
};
