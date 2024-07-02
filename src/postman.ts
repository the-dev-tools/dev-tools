import * as S from '@effect/schema/Schema';

// Generated using: https://app.quicktype.io
// Documentation: https://learning.postman.com/collection-format
// JSON Schema: https://schema.postman.com/collection/json/v2.1.0/draft-07/collection.json

const DEFAULT_NAME = 'API Recorder Collection';
const DEFAULT_SCHEMA = 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json';

export const AuthType = S.Literal(
  'apikey',
  'awsv4',
  'basic',
  'bearer',
  'digest',
  'edgegrid',
  'hawk',
  'noauth',
  'ntlm',
  'oauth1',
  'oauth2',
);
export type AuthType = S.Schema.Type<typeof AuthType>;

export const VariableType = S.Literal('any', 'boolean', 'number', 'string');
export type VariableType = S.Schema.Type<typeof VariableType>;

export const FormParameterType = S.Literal('file', 'text');
export type FormParameterType = S.Schema.Type<typeof FormParameterType>;

export const Mode = S.Literal('file', 'formdata', 'graphql', 'raw', 'urlencoded');
export type Mode = S.Schema.Type<typeof Mode>;

export class Cookie extends S.Class<Cookie>('Cookie')({
  domain: S.String,
  expires: S.optional(S.Union(S.Null, S.String)),
  extensions: S.optional(S.Union(S.Array(S.Any), S.Null)),
  hostOnly: S.optional(S.Union(S.Boolean, S.Null)),
  httpOnly: S.optional(S.Union(S.Boolean, S.Null)),
  maxAge: S.optional(S.Union(S.Null, S.String)),
  name: S.optional(S.Union(S.Null, S.String)),
  path: S.String,
  secure: S.optional(S.Union(S.Boolean, S.Null)),
  session: S.optional(S.Union(S.Boolean, S.Null)),
  value: S.optional(S.Union(S.Null, S.String)),
}) {}

export class Response extends S.Class<Response>('Response')({
  body: S.optional(S.Union(S.Null, S.String)),
  code: S.optional(S.Union(S.Number, S.Null)),
  cookie: S.optional(S.Union(S.Array(Cookie), S.Null)),
  header: S.optional(
    S.Union(
      S.Array(
        S.Union(
          S.suspend(() => Header),
          S.String,
        ),
      ),
      S.Null,
      S.String,
    ),
  ),
  id: S.optional(S.Union(S.Null, S.String)),
  originalRequest: S.optional(
    S.Union(
      S.suspend(() => RequestClass),
      S.Null,
      S.String,
    ),
  ),
  responseTime: S.optional(S.Union(S.Number, S.Null, S.String)),
  status: S.optional(S.Union(S.Null, S.String)),
  timings: S.optional(S.Union(S.Record(S.String, S.Any), S.Null)),
}) {}

export class ProxyConfig extends S.Class<ProxyConfig>('ProxyConfig')({
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  host: S.optional(S.Union(S.Null, S.String)),
  match: S.optional(S.Union(S.Null, S.String)),
  port: S.optional(S.Union(S.Number, S.Null)),
  tunnel: S.optional(S.Union(S.Boolean, S.Null)),
}) {}

export class Header extends S.Class<Header>('Header')({
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  key: S.String,
  value: S.String,
}) {}

export class Key extends S.Class<Key>('Key')({
  src: S.optional(S.Any),
}) {}

export class Cert extends S.Class<Cert>('Cert')({
  src: S.optional(S.Any),
}) {}

export class Certificate extends S.Class<Certificate>('Certificate')({
  cert: S.optional(S.Union(Cert, S.Null)),
  key: S.optional(S.Union(Key, S.Null)),
  matches: S.optional(S.Union(S.Array(S.String), S.Null)),
  name: S.optional(S.Union(S.Null, S.String)),
  passphrase: S.optional(S.Union(S.Null, S.String)),
}) {}

export class UrlEncodedParameter extends S.Class<UrlEncodedParameter>('UrlEncodedParameter')({
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  key: S.String,
  value: S.optional(S.Union(S.Null, S.String)),
}) {}

export class FormParameter extends S.Class<FormParameter>('FormParameter')({
  contentType: S.optional(S.Union(S.Null, S.String)),
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  key: S.String,
  type: S.optional(S.Union(FormParameterType, S.Null)),
  value: S.optional(S.Union(S.Null, S.String)),
  src: S.optional(S.Union(S.Array(S.Any), S.Null, S.String)),
}) {}

export class File extends S.Class<File>('File')({
  content: S.optional(S.Union(S.Null, S.String)),
  src: S.optional(S.Union(S.Null, S.String)),
}) {}

export class Body extends S.Class<Body>('Body')({
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  file: S.optional(S.Union(File, S.Null)),
  formdata: S.optional(S.Union(S.Array(FormParameter), S.Null)),
  graphql: S.optional(S.Union(S.Record(S.String, S.Any), S.Null)),
  mode: S.optional(S.Union(Mode, S.Null)),
  options: S.optional(S.Union(S.Record(S.String, S.Any), S.Null)),
  raw: S.optional(S.Union(S.Null, S.String)),
  urlencoded: S.optional(S.Union(S.Array(UrlEncodedParameter), S.Null)),
}) {}

export class RequestClass extends S.Class<RequestClass>('RequestClass')({
  auth: S.optional(
    S.Union(
      S.suspend(() => Auth),
      S.Null,
    ),
  ),
  body: S.optional(S.Union(Body, S.Null)),
  certificate: S.optional(S.Union(Certificate, S.Null)),
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  header: S.optional(S.Union(S.Array(Header), S.Null, S.String)),
  method: S.optional(S.Union(S.Null, S.String)),
  proxy: S.optional(S.Union(ProxyConfig, S.Null)),
  url: S.optional(
    S.Union(
      S.suspend(() => UrlClass),
      S.Null,
      S.String,
    ),
  ),
}) {}

export class Item extends S.Class<Item>('Item')({
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  event: S.optional(S.Union(S.Array(S.suspend(() => Event)), S.Null)),
  id: S.optional(S.Union(S.Null, S.String)),
  name: S.optional(S.Union(S.Null, S.String)),
  protocolProfileBehavior: S.optional(S.Union(S.Record(S.String, S.Any), S.Null)),
  request: S.optional(S.Union(RequestClass, S.Null, S.String)),
  response: S.optional(S.Union(S.Array(Response), S.Null)),
  variable: S.optional(S.Union(S.Array(S.suspend(() => Variable)), S.Null)),
  auth: S.optional(
    S.Union(
      S.suspend(() => Auth),
      S.Null,
    ),
  ),
  item: S.optional(S.Union(S.Array(S.suspend((): S.Schema<Item> => Item)), S.Null)),
}) {}

export class CollectionVersionClass extends S.Class<CollectionVersionClass>('CollectionVersionClass')({
  identifier: S.optional(S.Union(S.Null, S.String)),
  major: S.Number,
  meta: S.optional(S.Any),
  minor: S.Number,
  patch: S.Number,
}) {}

export class Information extends S.Class<Information>('Information')({
  _postman_id: S.optional(S.Union(S.Null, S.String)),
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  name: S.String,
  schema: S.String,
  version: S.optional(S.Union(CollectionVersionClass, S.Null, S.String)),
}) {}

export class Variable extends S.Class<Variable>('Variable')({
  description: S.optional(
    S.Union(
      S.suspend(() => Description),
      S.Null,
      S.String,
    ),
  ),
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  id: S.optional(S.Union(S.Null, S.String)),
  key: S.optional(S.Union(S.Null, S.String)),
  name: S.optional(S.Union(S.Null, S.String)),
  system: S.optional(S.Union(S.Boolean, S.Null)),
  type: S.optional(S.Union(VariableType, S.Null)),
  value: S.optional(S.Any),
}) {}

export class Description extends S.Class<Description>('Description')({
  content: S.optional(S.Union(S.Null, S.String)),
  type: S.optional(S.Union(S.Null, S.String)),
  version: S.optional(S.Any),
}) {}

export class QueryParam extends S.Class<QueryParam>('QueryParam')({
  description: S.optional(S.Union(Description, S.Null, S.String)),
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  key: S.optional(S.Union(S.Null, S.String)),
  value: S.optional(S.Union(S.Null, S.String)),
}) {}

export class PathClass extends S.Class<PathClass>('PathClass')({
  type: S.optional(S.Union(S.Null, S.String)),
  value: S.optional(S.Union(S.Null, S.String)),
}) {}

export class UrlClass extends S.Class<UrlClass>('UrlClass')({
  hash: S.optional(S.Union(S.Null, S.String)),
  host: S.optional(S.Union(S.Array(S.String), S.Null, S.String)),
  path: S.optional(S.Union(S.Array(S.Union(PathClass, S.String)), S.Null, S.String)),
  port: S.optional(S.Union(S.Null, S.String)),
  protocol: S.optional(S.Union(S.Null, S.String)),
  query: S.optional(S.Union(S.Array(QueryParam), S.Null)),
  raw: S.optional(S.Union(S.Null, S.String)),
  variable: S.optional(S.Union(S.Array(Variable), S.Null)),
}) {}

export class Script extends S.Class<Script>('Script')({
  exec: S.optional(S.Union(S.Array(S.String), S.Null, S.String)),
  id: S.optional(S.Union(S.Null, S.String)),
  name: S.optional(S.Union(S.Null, S.String)),
  src: S.optional(S.Union(UrlClass, S.Null, S.String)),
  type: S.optional(S.Union(S.Null, S.String)),
}) {}

export class Event extends S.Class<Event>('Event')({
  disabled: S.optional(S.Union(S.Boolean, S.Null)),
  id: S.optional(S.Union(S.Null, S.String)),
  listen: S.String,
  script: S.optional(S.Union(Script, S.Null)),
}) {}

export class ApikeyElement extends S.Class<ApikeyElement>('ApikeyElement')({
  key: S.String,
  type: S.optional(S.Union(S.Null, S.String)),
  value: S.optional(S.Any),
}) {}

export class Auth extends S.Class<Auth>('Auth')({
  apikey: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  awsv4: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  basic: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  bearer: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  digest: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  edgegrid: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  hawk: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  noauth: S.optional(S.Any),
  ntlm: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  oauth1: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  oauth2: S.optional(S.Union(S.Array(ApikeyElement), S.Null)),
  type: AuthType,
}) {}

export class Collection extends S.Class<Collection>('Collection')({
  auth: S.optional(S.Union(Auth, S.Null)),
  event: S.optional(S.Union(S.Array(Event), S.Null)),
  info: Information.pipe(
    S.propertySignature,
    S.withConstructorDefault(() => new Information({ name: DEFAULT_NAME, schema: DEFAULT_SCHEMA })),
  ),
  item: S.optional(S.Array(Item)).pipe(S.withDefaults({ decoding: () => [], constructor: () => [] })),
  protocolProfileBehavior: S.optional(S.Union(S.Record(S.String, S.Any), S.Null)),
  variable: S.optional(S.Union(S.Array(Variable), S.Null)),
}) {}
