import CodeMirror from '@uiw/react-codemirror';
import {
  GraphQLCollectionSchema,
  GraphQLDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { useDeltaState } from '~/features/delta';

export interface GraphQLQueryEditorProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
  isReadOnly?: boolean;
}

export const GraphQLQueryEditor = ({
  deltaGraphqlId,
  graphqlId,
  isReadOnly = false,
}: GraphQLQueryEditorProps) => {
  const { theme } = useTheme();

  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
    valueKey: 'query',
  } as const;

  const [value, setValue] = useDeltaState(deltaOptions);

  return (
    <CodeMirror
      className={tw`h-full`}
      height='100%'
      indentWithTab={false}
      onChange={(_) => void setValue(_)}
      placeholder='Enter your GraphQL query...'
      readOnly={isReadOnly}
      theme={theme}
      value={value ?? ''}
    />
  );
};
