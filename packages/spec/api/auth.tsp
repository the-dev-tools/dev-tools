namespace API.Auth;

model AuthTokens {
  accessToken: string;
  refreshToken: string;
}

model AuthMagicLinkRequest {
  /** https://magic.link/docs/authentication/features/decentralized-id */
  didToken: string;
}
model AuthMagicLinkResponse is AuthTokens;

model AuthRefreshRequest {
  refreshToken: string;
}
model AuthRefreshResponse is AuthTokens;

interface AuthService {
  AuthMagicLink(...AuthMagicLinkRequest): AuthMagicLinkResponse;
  AuthRefresh(...AuthRefreshRequest): AuthRefreshResponse;
}
