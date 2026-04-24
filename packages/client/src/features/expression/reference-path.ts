import { ReferenceKey, ReferenceKeyKind } from '@the-dev-tools/spec/buf/api/reference/v1/reference_pb';

/**
 * Strip the leading "env" GROUP key — the reference tree nests env vars under
 * a GROUP "env", but the expression engine resolves them at root level.
 */
const stripEnvGroup = (keys: ReferenceKey[]): ReferenceKey[] =>
  keys.length > 1 && keys[0].kind === ReferenceKeyKind.GROUP && keys[0].group === 'env' ? keys.slice(1) : keys;

/** Convert reference keys to a dot-separated path: `http_4.response.body.token` */
export const referenceKeysToPath = (keys: ReferenceKey[]): string => {
  let path = '';
  for (const key of stripEnvGroup(keys)) {
    switch (key.kind) {
      case ReferenceKeyKind.ANY:
        path += '[*]';
        break;
      case ReferenceKeyKind.GROUP:
        if (path) path += '.';
        path += key.group ?? '';
        break;
      case ReferenceKeyKind.INDEX:
        path += `[${String(key.index)}]`;
        break;
      case ReferenceKeyKind.KEY:
        if (path) path += '.';
        path += key.key ?? '';
        break;
    }
  }
  return path;
};

/** Convert reference keys to an expression string based on the editor kind */
export const referenceKeysToExpression = (
  keys: ReferenceKey[],
  kind: 'FullExpression' | 'StringExpression',
): string => {
  const path = referenceKeysToPath(keys);
  return kind === 'StringExpression' ? `{{ ${path} }}` : path;
};

/** Convert reference keys to a JS expression: `ctx["http_4"].response.body.token` */
export const referenceKeysToJsExpression = (keys: ReferenceKey[]): string => {
  const resolved = stripEnvGroup(keys);
  let result = '';
  for (let i = 0; i < resolved.length; i++) {
    const key = resolved[i];
    switch (key.kind) {
      case ReferenceKeyKind.ANY:
        result += '[*]';
        break;
      case ReferenceKeyKind.GROUP:
        result += i === 0 ? `ctx["${key.group ?? ''}"]` : `.${key.group ?? ''}`;
        break;
      case ReferenceKeyKind.INDEX:
        result += `[${String(key.index)}]`;
        break;
      case ReferenceKeyKind.KEY:
        result += i === 0 ? `ctx["${key.key ?? ''}"]` : `.${key.key ?? ''}`;
        break;
    }
  }
  return result;
};
