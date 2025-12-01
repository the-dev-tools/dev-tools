import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { LuTrash2 } from 'react-icons/lu';
import {
  HttpAssertCollectionSchema,
  HttpAssertDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceField } from '~/reference';
import { DeltaResetButton, useDeltaColumnState } from '~/utils/delta';
import { eqStruct, pick } from '~/utils/tanstack-db';

export interface AssertPanelProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const AssertPanel = ({ deltaHttpId, httpId, isReadOnly = false }: AssertPanelProps) => {
  const collection = useApiCollection(HttpAssertCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .select((_) => pick(_.item, 'httpAssertId')),
    [collection, deltaHttpId, httpId],
  ).data;

  return (
    <div className={tw`flex flex-col gap-2`}>
      {items.map((_) => (
        <Assert
          deltaHttpId={deltaHttpId}
          httpAssertId={_.httpAssertId}
          isReadOnly={isReadOnly}
          key={collection.utils.getKey(_)}
        />
      ))}

      {!isReadOnly && (
        <Button
          onPress={() =>
            void collection.utils.insert({
              httpAssertId: Ulid.generate().bytes,
              httpId: deltaHttpId ?? httpId,
            })
          }
        >
          New Assertion
        </Button>
      )}
    </div>
  );
};

interface AssertProps {
  deltaHttpId: Uint8Array | undefined;
  httpAssertId: Uint8Array;
  isReadOnly?: boolean;
}

const Assert = ({ deltaHttpId, httpAssertId, isReadOnly = false }: AssertProps) => {
  const collection = useApiCollection(HttpAssertCollectionSchema);

  const { deltaOptions, setValue, value } = useDeltaColumnState({
    deltaKey: 'deltaHttpAssertId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpAssertDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpAssertId',
    originKeyObject: { httpAssertId },
    originSchema: HttpAssertCollectionSchema,
    valueKey: 'value',
  });

  const isExtra =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ httpAssertId }))
          .select((_) => ({ isExtra: eqStruct({ httpId: deltaHttpId })(_) }))
          .findOne(),
      [collection, deltaHttpId, httpAssertId],
    ).data?.isExtra ?? false;

  return (
    <div className={tw`flex items-center gap-2`}>
      <ReferenceField
        allowFiles
        className={tw`flex-1`}
        onChange={(_) => void setValue(_ as never)}
        placeholder='Enter value to compare'
        readOnly={isReadOnly}
        value={value as unknown as string}
      />

      {!isReadOnly && (deltaHttpId === undefined || isExtra) && (
        <Button
          className={tw`h-8 text-red-700`}
          onPress={() => void collection.utils.delete({ httpAssertId })}
          variant='secondary'
        >
          <LuTrash2 />
        </Button>
      )}

      {!isReadOnly && <DeltaResetButton {...deltaOptions} />}
    </div>
  );
};
