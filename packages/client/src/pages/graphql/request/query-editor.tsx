import CodeMirror from '@uiw/react-codemirror';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';

export interface GraphQLQueryEditorProps {
  graphqlId: Uint8Array;
}

export const GraphQLQueryEditor = ({ graphqlId }: GraphQLQueryEditorProps) => {
  const collection = useApiCollection(GraphQLCollectionSchema);
  const item = collection.get(collection.utils.getKey({ graphqlId }));

  return (
    <CodeMirror
      className={tw`h-full`}
      height='100%'
      indentWithTab={false}
      onChange={(value) => collection.utils.update({ graphqlId, query: value })}
      placeholder='Enter your GraphQL query...'
      value={item?.query ?? ''}
    />
  );
};
