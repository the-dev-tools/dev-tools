import { Match, pipe } from 'effect';
import { HttpBodyKind } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { DeltaResetButton, useDeltaState } from '~/features/delta';
import { BodyFormDataTable } from './form-data';
import { RawForm } from './raw';
import { BodyUrlEncodedTable } from './url-encoded';

export interface BodyPanelProps {
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const BodyPanel = ({ deltaHttpId, hideDescription = false, httpId, isReadOnly = false }: BodyPanelProps) => {
  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
    valueKey: 'bodyKind',
  } as const;

  const [bodyKind, setBodyKind] = useDeltaState(deltaOptions);

  return (
    <div className={tw`grid h-full flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4`}>
      <div className={tw`flex items-center gap-2`}>
        <RadioGroup
          aria-label='Body type'
          className={tw`h-7 justify-center`}
          isReadOnly={isReadOnly}
          onChange={(key) => void setBodyKind(parseInt(key))}
          orientation='horizontal'
          value={(bodyKind ?? 0).toString()}
        >
          <Radio value={HttpBodyKind.UNSPECIFIED.toString()}>none</Radio>
          <Radio value={HttpBodyKind.FORM_DATA.toString()}>form-data</Radio>
          <Radio value={HttpBodyKind.URL_ENCODED.toString()}>x-www-form-urlencoded</Radio>
          <Radio value={HttpBodyKind.RAW.toString()}>raw</Radio>
        </RadioGroup>

        <DeltaResetButton {...deltaOptions} />
      </div>

      {pipe(
        Match.value(bodyKind),
        Match.when(HttpBodyKind.FORM_DATA, () => (
          <BodyFormDataTable
            deltaHttpId={deltaHttpId}
            hideDescription={hideDescription}
            httpId={httpId}
            isReadOnly={isReadOnly}
          />
        )),
        Match.when(HttpBodyKind.URL_ENCODED, () => (
          <BodyUrlEncodedTable
            deltaHttpId={deltaHttpId}
            hideDescription={hideDescription}
            httpId={httpId}
            isReadOnly={isReadOnly}
          />
        )),
        Match.when(HttpBodyKind.RAW, () => (
          <RawForm deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />
        )),
        Match.orElse(() => null),
      )}
    </div>
  );
};
