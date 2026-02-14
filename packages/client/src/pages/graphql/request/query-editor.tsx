import CodeMirror from '@uiw/react-codemirror';
import { useState } from 'react';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { useApiCollection } from '~/shared/api';

export interface GraphQLQueryEditorProps {
  graphqlId: Uint8Array;
}

export const GraphQLQueryEditor = ({ graphqlId }: GraphQLQueryEditorProps) => {
  const { theme } = useTheme();
  const collection = useApiCollection(GraphQLCollectionSchema);
  const item = collection.get(collection.utils.getKey({ graphqlId }));
  const [localQuery, setLocalQuery] = useState<string>();

  return (
    <CodeMirror
      className={tw`h-full`}
      height='100%'
      indentWithTab={false}
      onChange={(value) => {
        setLocalQuery(value);
        collection.utils.updatePaced({ graphqlId, query: value });
      }}
      placeholder='Enter your GraphQL query...'
      theme={theme}
      value={localQuery ?? item?.query ?? ''}
    />
  );
};
