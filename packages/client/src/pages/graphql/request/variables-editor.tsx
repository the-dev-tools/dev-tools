import { json } from '@codemirror/lang-json';
import CodeMirror from '@uiw/react-codemirror';
import { useMemo } from 'react';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';

export interface GraphQLVariablesEditorProps {
  graphqlId: Uint8Array;
}

export const GraphQLVariablesEditor = ({ graphqlId }: GraphQLVariablesEditorProps) => {
  const collection = useApiCollection(GraphQLCollectionSchema);
  const item = collection.get(collection.utils.getKey({ graphqlId }));

  const extensions = useMemo(() => [json()], []);

  return (
    <CodeMirror
      className={tw`h-full`}
      extensions={extensions}
      height='100%'
      indentWithTab={false}
      onChange={(value) => collection.utils.update({ graphqlId, variables: value })}
      placeholder='{"key": "value"}'
      value={item?.variables ?? ''}
    />
  );
};
