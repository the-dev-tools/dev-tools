import * as PS from '@plasmohq/storage';

export const Local = new PS.Storage({ area: 'local' });

export const RECORDING_TAB_ID = 'RECORDING_TAB_ID';

export const RECORDED_CALLS = 'RECORDED_CALLS';

export interface NetworkCall {
  method: string;
  url: string;
  time: number;
}
