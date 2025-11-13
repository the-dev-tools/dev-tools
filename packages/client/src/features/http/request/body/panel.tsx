import { eq, useLiveQuery } from '@tanstack/react-db';
import { Match, pipe } from 'effect';
import { HttpBodyKind } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { pick } from '~/utils/tanstack-db';
import { FormDataTable } from './form-data';
import { RawForm } from './raw';
import { UrlEncodedTable } from './url-encoded';

export interface BodyPanelProps {
  httpId: Uint8Array;
}

export const BodyPanel = ({ httpId }: BodyPanelProps) => {
  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { data: { bodyKind = HttpBodyKind.UNSPECIFIED } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: httpCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => pick(_.item, 'bodyKind'))
        .findOne(),
    [httpCollection, httpId],
  );

  return (
    <div className={tw`grid h-full flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4`}>
      <RadioGroup
        aria-label='Body type'
        className={tw`h-7 justify-center`}
        onChange={(key) => httpCollection.utils.update({ bodyKind: parseInt(key), httpId })}
        orientation='horizontal'
        value={bodyKind.toString()}
      >
        <Radio value={HttpBodyKind.UNSPECIFIED.toString()}>none</Radio>
        <Radio value={HttpBodyKind.FORM_DATA.toString()}>form-data</Radio>
        <Radio value={HttpBodyKind.URL_ENCODED.toString()}>x-www-form-urlencoded</Radio>
        <Radio value={HttpBodyKind.RAW.toString()}>raw</Radio>
      </RadioGroup>

      {pipe(
        Match.value(bodyKind),
        Match.when(HttpBodyKind.FORM_DATA, () => <FormDataTable httpId={httpId} />),
        Match.when(HttpBodyKind.URL_ENCODED, () => <UrlEncodedTable httpId={httpId} />),
        Match.when(HttpBodyKind.RAW, () => <RawForm httpId={httpId} />),
        Match.orElse(() => null),
      )}
    </div>
  );
};
