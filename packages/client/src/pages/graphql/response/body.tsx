import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useQuery } from '@tanstack/react-query';
import CodeMirror from '@uiw/react-codemirror';
import { GraphQLResponseSchema } from '@the-dev-tools/spec/buf/api/graph_q_l/v1/graph_q_l_pb';
import { GraphQLResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { prettierFormatQueryOptions, useCodeMirrorLanguageExtensions } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';

export interface GraphQLResponseBodyProps {
  graphqlResponseId: Uint8Array;
}

export const GraphQLResponseBody = ({ graphqlResponseId }: GraphQLResponseBodyProps) => {
  const { theme } = useTheme();
  const collection = useApiCollection(GraphQLResponseCollectionSchema);

  const { body } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.graphqlResponseId, graphqlResponseId))
          .select((_) => pick(_.item, 'body'))
          .findOne(),
      [collection, graphqlResponseId],
    ).data ?? create(GraphQLResponseSchema);

  const { data: prettierBody } = useQuery(prettierFormatQueryOptions({ language: 'json', text: body }));
  const extensions = useCodeMirrorLanguageExtensions('json');

  return (
    <CodeMirror
      className={tw`flex-1`}
      extensions={extensions}
      height='100%'
      indentWithTab={false}
      readOnly
      theme={theme}
      value={prettierBody}
    />
  );
};
