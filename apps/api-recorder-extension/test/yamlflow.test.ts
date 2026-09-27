import type { Protocol } from 'devtools-protocol';

import { Effect } from 'effect';
import { expect, test } from 'vitest';
import { parse } from 'yaml';

import * as Postman from '~postman';
import * as Recorder from '~recorder';
import * as YamlFlow from '~yamlflow';

// Replays a small browsing session through the same recorder functions the
// background worker uses, then exports it. The resulting YAML is written to
// test/fixtures/recording.yamlflow.yaml, which the server's Go test
// `TestAPIRecorderExtensionExport` imports with the real yamlflow parser
// (packages/server/pkg/translate/yamlflowsimplev2/extension_export_test.go).

interface SampleCall {
  body?: string;
  headers: Record<string, string>;
  method: string;
  status: number;
  url: string;
}

const calls: SampleCall[] = [
  {
    headers: {
      ':authority': 'api.example.com',
      Accept: 'application/json',
      Authorization: 'Bearer demo-token',
      'sec-ch-ua-mobile': '?0',
      'Sec-Fetch-Mode': 'cors',
    },
    method: 'GET',
    status: 200,
    url: 'https://api.example.com/v1/users?page=1&limit=20',
  },
  {
    body: '{"name":"Ada Lovelace","email":"ada@example.com"}',
    headers: {
      Accept: 'application/json',
      Authorization: 'Bearer demo-token',
      'Content-Length': '48',
      'Content-Type': 'application/json',
    },
    method: 'POST',
    status: 201,
    url: 'https://api.example.com/v1/users',
  },
  {
    headers: { Accept: 'application/json' },
    method: 'GET',
    status: 304,
    url: 'https://api.example.com/v1/users/42',
  },
  {
    headers: { Accept: 'application/json' },
    method: 'GET',
    status: 200,
    url: 'https://api.example.com/v1/search?tag=a&tag=b',
  },
  {
    headers: { Accept: 'application/json', Authorization: 'Bearer demo-token' },
    method: 'GET',
    status: 200,
    url: 'https://api.example.com/v1/users?page=2&limit=20',
  },
];

const record = Effect.gen(function* () {
  const indexMap = Recorder.makeIndexMap();
  let collection = new Postman.Collection();

  const tab = { url: 'https://app.example.com/dashboard' } as chrome.tabs.Tab;
  collection = yield* Recorder.addNavigation(collection, tab);

  for (const [index, call] of calls.entries()) {
    const requestId = `request-${index.toString()}`;

    const requestEvent = {
      request: { headers: call.headers, method: call.method, url: call.url },
      requestId,
      wallTime: 1_790_000_000 + index,
    } as unknown as Protocol.Network.RequestWillBeSentEvent;
    collection = yield* Recorder.addRequest(
      collection,
      indexMap,
      requestEvent,
      call.body === undefined ? {} : { postData: call.body },
    );

    const responseEvent = {
      requestId,
      response: { headers: { 'Content-Type': 'application/json' }, status: call.status, statusText: '', url: call.url },
    } as unknown as Protocol.Network.ResponseReceivedEvent;
    collection = yield* Recorder.addResponse(collection, indexMap, responseEvent, { body: '{}' });
  }

  return collection;
});

test('exports a recording as a DevTools YAML flow', async () => {
  const collection = await Effect.runPromise(record);
  const yaml = YamlFlow.toYamlFlow(collection);

  await expect(yaml).toMatchFileSnapshot('./fixtures/recording.yamlflow.yaml');

  const document = parse(yaml) as YamlFlow.YamlFlowDocument;
  expect(document.version).toBe(2);
  expect(document.run).toEqual([{ flow: 'Recorded flow' }]);
  expect(document.requests?.map((_) => _.name)).toEqual([
    'get_v1_users',
    'post_v1_users',
    'get_v1_users_42',
    'get_v1_search',
    'get_v1_users_2',
  ]);

  const [list, create, cached, search] = document.requests ?? [];
  expect(list).toMatchObject({
    assertions: ['response.status == 200'],
    query_params: { limit: '20', page: '1' },
    url: 'https://api.example.com/v1/users',
  });
  expect(Object.keys(list?.headers ?? {})).toEqual(['Accept', 'Authorization']);
  expect(create).toMatchObject({ body: calls[1]?.body, method: 'POST' });
  expect(create?.headers).not.toHaveProperty('Content-Length');
  expect(cached?.assertions).toBeUndefined();
  expect(search?.query_params).toEqual([
    { enabled: true, name: 'tag', value: 'a' },
    { enabled: true, name: 'tag', value: 'b' },
  ]);

  // Steps run one after another, in recording order
  const steps = document.flows[0]?.steps ?? [];
  expect(steps[0]).toEqual({ manual_start: { name: 'Start' } });
  expect(steps.slice(1).map((_) => ('request' in _ ? _.request.depends_on : undefined))).toEqual([
    'Start',
    'get_v1_users',
    'post_v1_users',
    'get_v1_users_42',
    'get_v1_search',
  ]);
});

test('exports an empty recording as a valid flow with only a start step', () => {
  const document = YamlFlow.toYamlFlowDocument([]);
  expect(document.requests).toBeUndefined();
  expect(document.flows).toEqual([{ name: 'Recorded flow', steps: [{ manual_start: { name: 'Start' } }] }]);
});
