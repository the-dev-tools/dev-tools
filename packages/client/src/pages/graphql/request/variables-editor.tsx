import { json } from '@codemirror/lang-json';
import CodeMirror from '@uiw/react-codemirror';
import { useMemo } from 'react';
import {
  GraphQLCollectionSchema,
  GraphQLDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { useDeltaState } from '~/features/delta';

export interface GraphQLVariablesEditorProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
  isReadOnly?: boolean;
}

export const GraphQLVariablesEditor = ({
  deltaGraphqlId,
  graphqlId,
  isReadOnly = false,
}: GraphQLVariablesEditorProps) => {
  const { theme } = useTheme();

  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
    valueKey: 'variables',
  } as const;

  const [value, setValue] = useDeltaState(deltaOptions);

  const extensions = useMemo(() => [json()], []);

  return (
    <CodeMirror
      className={tw`h-full`}
      extensions={extensions}
      height='100%'
      indentWithTab={false}
      onChange={(_) => void setValue(_)}
      placeholder='{"key": "value"}'
      readOnly={isReadOnly}
      theme={theme}
      value={value ?? ''}
    />
  );
};
