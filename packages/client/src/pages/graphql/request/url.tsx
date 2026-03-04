import {
  GraphQLCollectionSchema,
  GraphQLDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { DeltaResetButton, useDeltaState } from '~/features/delta';
import { ReferenceField } from '~/features/expression';

export interface GraphQLUrlProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
  isReadOnly?: boolean;
}

export const GraphQLUrl = ({ deltaGraphqlId, graphqlId, isReadOnly = false }: GraphQLUrlProps) => {
  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
  };

  const [url, setUrl] = useDeltaState({ ...deltaOptions, valueKey: 'url' });

  return (
    <div className={tw`flex flex-1 items-center gap-3 rounded-lg border border-neutral px-3 py-2 shadow-xs`}>
      <ReferenceField
        aria-label='GraphQL Endpoint URL'
        className={tw`min-w-0 flex-1 border-none font-medium tracking-tight`}
        kind='StringExpression'
        onChange={(_) => void setUrl(_)}
        readOnly={isReadOnly}
        value={url ?? ''}
      />
      <DeltaResetButton {...deltaOptions} valueKey='url' />
    </div>
  );
};
