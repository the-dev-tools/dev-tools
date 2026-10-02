import { stringify } from 'yaml';

import * as Postman from '~postman';

// Exports a recording as a DevTools YAML flow ("yamlflow"), the format Studio
// imports and the CLI runs (`devtools flow run <file>`).
//
// The shape mirrors the server's own schema and exporter, and uses no other fields:
//   packages/server/pkg/translate/yamlflowsimplev2/types.go     (YamlFlowFormatV2)
//   packages/server/pkg/translate/yamlflowsimplev2/testdata/golden/*.golden
// The Go test `TestAPIRecorderExtensionExport` in that package imports the
// fixture produced by `yamlflow.test.ts`, so a drift on either side fails CI.

export const YAMLFLOW_VERSION = 2;

/* eslint-disable perfectionist/sort-objects -- key order is the YAML field order, kept identical to the Go exporter */

const START_STEP = 'Start';

interface RequestDef {
  assertions?: string[];
  body?: string;
  headers?: Record<string, string>;
  method: string;
  name: string;
  query_params?: NameValue[] | Record<string, string>;
  url: string;
}

interface NameValue {
  enabled: boolean;
  name: string;
  value: string;
}

type Step = { manual_start: { name: string } } | { request: { depends_on: string; name: string; use_request: string } };

export interface YamlFlowDocument {
  flows: { name: string; steps: Step[] }[];
  requests?: RequestDef[];
  run: { flow: string }[];
  version: number;
  workspace_name: string;
}

export interface RecordedRequest {
  body?: string;
  headers: (readonly [string, string])[];
  method: string;
  status?: number;
  timestamp?: number;
  url: string;
}

/** Flattens the recorder's host > page > request tree into requests in the order they were sent */
export const collectRequests = (collection: Postman.Collection): RecordedRequest[] => {
  const items: Postman.Item[] = [];
  for (const host of collection.item) {
    for (const page of host.item ?? []) {
      for (const request of page.item ?? []) items.push(request);
    }
  }

  // The recorder prepends new entries, so the tree is newest-first
  const requests = items.reverse().flatMap((item): RecordedRequest[] => {
    const request = item.request;
    if (!(request instanceof Postman.RequestClass)) return [];

    const url = typeof request.url === 'string' ? request.url : (request.url?.raw ?? undefined);
    if (!url) return [];

    const headers = Array.isArray(request.header)
      ? request.header.map((header: Postman.Header) => [header.key, header.value] as const)
      : [];

    const timestamp = item.variable?.find((_) => _.key === 'timestamp')?.value as unknown;
    const status = item.response?.[0]?.code ?? undefined;
    const body = request.body?.raw ?? undefined;

    return [
      {
        headers,
        method: (request.method ?? 'GET').toUpperCase(),
        url,
        ...(body ? { body } : {}),
        ...(typeof status === 'number' ? { status } : {}),
        ...(typeof timestamp === 'number' ? { timestamp } : {}),
      },
    ];
  });

  // Requests from different pages interleave, so order by wall time when we have it
  if (requests.every((_) => _.timestamp !== undefined)) {
    return requests
      .map((request, index) => ({ index, request }))
      .sort((a, b) => (a.request.timestamp ?? 0) - (b.request.timestamp ?? 0) || a.index - b.index)
      .map((_) => _.request);
  }
  return requests;
};

// Headers the HTTP client sets itself (replaying a stale value breaks requests),
// plus browser-generated client hints and fetch metadata, which only add noise
const isSkippedHeader = (name: string) => {
  const key = name.toLowerCase();
  return key.startsWith(':') || key === 'content-length' || key.startsWith('sec-ch-') || key.startsWith('sec-fetch-');
};

const makeStepName = (method: string, url: URL, taken: Set<string>) => {
  const segments = url.pathname.split('/').filter(Boolean).slice(-3);
  const base =
    [method.toLowerCase(), ...segments]
      .join('_')
      .replace(/[^A-Za-z0-9_]+/g, '_')
      .replace(/_+/g, '_')
      .replace(/^_|_$/g, '')
      .slice(0, 48)
      .replace(/_$/, '') || 'request';

  let name = base;
  for (let i = 2; taken.has(name.toLowerCase()); i++) name = `${base}_${i.toString()}`;
  taken.add(name.toLowerCase());
  return name;
};

const toQueryParams = (params: URLSearchParams) => {
  const entries = [...params.entries()];
  if (entries.length === 0) return undefined;
  const keys = new Set(entries.map(([key]) => key));
  // The map form is the canonical one, but it can't hold repeated keys
  if (keys.size === entries.length) return Object.fromEntries(entries);
  return entries.map(([name, value]): NameValue => ({ name, value, enabled: true }));
};

export interface ToYamlFlowOptions {
  flowName?: string;
  workspaceName?: string;
}

export const toYamlFlowDocument = (
  requests: RecordedRequest[],
  { flowName = 'Recorded flow', workspaceName = 'API Recorder' }: ToYamlFlowOptions = {},
): YamlFlowDocument => {
  const taken = new Set([START_STEP.toLowerCase()]);
  const definitions: RequestDef[] = [];
  const steps: Step[] = [{ manual_start: { name: START_STEP } }];

  let previous = START_STEP;
  for (const request of requests) {
    let url: URL;
    try {
      url = new URL(request.url);
    } catch {
      continue;
    }

    const name = makeStepName(request.method, url, taken);

    const headers = Object.fromEntries(request.headers.filter(([key]) => !isSkippedHeader(key)));
    const queryParams = toQueryParams(url.searchParams);
    const isSuccess = request.status !== undefined && request.status >= 200 && request.status < 300;

    definitions.push({
      name,
      method: request.method,
      url: `${url.origin}${url.pathname}`,
      ...(Object.keys(headers).length > 0 ? { headers } : {}),
      ...(queryParams ? { query_params: queryParams } : {}),
      ...(request.body ? { body: request.body } : {}),
      ...(isSuccess ? { assertions: [`response.status == ${String(request.status)}`] } : {}),
    });

    steps.push({ request: { name, depends_on: previous, use_request: name } });
    previous = name;
  }

  return {
    version: YAMLFLOW_VERSION,
    workspace_name: workspaceName,
    run: [{ flow: flowName }],
    ...(definitions.length > 0 ? { requests: definitions } : {}),
    flows: [{ name: flowName, steps }],
  };
};

export const toYamlFlow = (collection: Postman.Collection, options?: ToYamlFlowOptions) =>
  stringify(toYamlFlowDocument(collectRequests(collection), options), {
    aliasDuplicateObjects: false,
    lineWidth: 0,
    singleQuote: true,
  });
/* eslint-enable perfectionist/sort-objects */
