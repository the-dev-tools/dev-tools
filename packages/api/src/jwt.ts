import { Schema } from 'effect';

export class JWTPayload extends Schema.Class<JWTPayload>('JWTPayload')({
  exp: Schema.transform(Schema.Number, Schema.DateFromSelf, {
    strict: true,
    decode: (_) => new Date(_ * 1000),
    encode: (_) => Math.floor(_.getTime() / 1000),
  }),
}) {}

export const accessTokenKey = 'AccessToken';
export class AccessTokenPayload extends JWTPayload.extend<AccessTokenPayload>('AccessTokenPayload')({
  token_type: Schema.Literal('access_token'),
  email: Schema.String,
}) {}

export const refreshTokenKey = 'RefreshToken';
export class RefreshTokenPayload extends JWTPayload.extend<RefreshTokenPayload>('RefreshTokenPayload')({
  token_type: Schema.Literal('refresh_token'),
}) {}
