using DevTools;

namespace Api.Internal.Auth;

enum AuthProvider {
  Email,
  Google,
}

model User {
  id: Id;
  email: string;
  name: string;
  image?: string;
  emailVerified: boolean;
  createdAt: Protobuf.WellKnown.Timestamp;
  updatedAt: Protobuf.WellKnown.Timestamp;
}

model Account {
  id: Id;
  userId: Id;
  provider: AuthProvider;
  providerAccountId: string;
  passwordHash?: string;
  accessToken?: string;
  oauthRefreshToken?: string;
  accessTokenExpiresAt?: Protobuf.WellKnown.Timestamp;
  scope?: string;
  idToken?: string;
  createdAt: Protobuf.WellKnown.Timestamp;
  updatedAt: Protobuf.WellKnown.Timestamp;
}

// -- Credential RPCs --

model CreateUserWithPasswordRequest {
  email: string;
  name: string;
  password: string;
}

model CreateUserWithPasswordResponse {
  user: User;
  account: Account;

  /** BetterAuth session token — auth-service uses this to call Token and returns it as refreshToken to the client. */
  sessionToken: string;
}

op CreateUserWithPassword(...CreateUserWithPasswordRequest): CreateUserWithPasswordResponse;

model VerifyCredentialsRequest {
  email: string;
  password: string;
}

model VerifyCredentialsResponse {
  valid: boolean;
  user?: User;
  account?: Account;

  /** BetterAuth session token — returned on successful verification. */
  sessionToken?: string;
}

op VerifyCredentials(...VerifyCredentialsRequest): VerifyCredentialsResponse;

// -- Token RPC --

model TokenRequest {
  /** BetterAuth session token — used to authenticate with BetterAuth's jwt() plugin to get a signed JWT. */
  sessionToken: string;
}

model TokenResponse {
  /** JWT access token signed by BetterAuth JWKS. */
  accessToken: string;
}

op Token(...TokenRequest): TokenResponse;

// -- OAuth RPCs --

model OAuthUrlRequest {
  provider: AuthProvider;
  callbackUrl: string;
  state?: string;
}

model OAuthUrlResponse {
  url: string;
  state: string;
}

op OAuthUrl(...OAuthUrlRequest): OAuthUrlResponse;

model ExchangeOAuthCodeRequest {
  provider: AuthProvider;
  code: string;
  state?: string;
}

model ExchangeOAuthCodeResponse {
  user: User;
  isNewUser: boolean;

  /** BetterAuth session token from the OAuth callback. */
  sessionToken: string;
}

op ExchangeOAuthCode(...ExchangeOAuthCodeRequest): ExchangeOAuthCodeResponse;

// -- Account RPCs --

model AccountsByUserIdRequest {
  userId: Id;
}

model AccountsByUserIdResponse {
  accounts: Account[];
  hasPassword: boolean;
}

op AccountsByUserId(...AccountsByUserIdRequest): AccountsByUserIdResponse;

model ListAllAccountsResponse {
  accounts: Account[];
}

op ListAllAccounts(): ListAllAccountsResponse;

model CreateAccountRequest {
  userId: Id;
  provider: AuthProvider;
  providerAccountId: string;
  password?: string;
  accessToken?: string;
  oauthRefreshToken?: string;
  accessTokenExpiresAt?: Protobuf.WellKnown.Timestamp;
  scope?: string;
  idToken?: string;
}

model CreateAccountResponse {
  account: Account;
}

op CreateAccount(...CreateAccountRequest): CreateAccountResponse;

// -- Session RPCs --

model RevokeSessionRequest {
  sessionToken: string;
}

model RevokeSessionResponse {}

op RevokeSession(...RevokeSessionRequest): RevokeSessionResponse;
