import * as Storage from '@/storage';

export {};

const recordRequest = async (details: chrome.webRequest.WebRequestBodyDetails) => {
  if (details.type !== 'xmlhttprequest') return;

  const recordingTabId = await Storage.Local.get<number>(Storage.RECORDING_TAB_ID);
  if (details.tabId !== recordingTabId) return;

  const recordedCalls = (await Storage.Local.get<Storage.NetworkCall[]>(Storage.RECORDED_CALLS)) ?? [];

  await Storage.Local.set(Storage.RECORDED_CALLS, [
    ...recordedCalls,
    {
      method: details.method,
      url: details.url,
      time: Date.now(),
    } satisfies Storage.NetworkCall,
  ]);
};

chrome.webRequest.onBeforeRequest.addListener((_) => void recordRequest(_), { urls: ['<all_urls>'] }, ['requestBody']);
