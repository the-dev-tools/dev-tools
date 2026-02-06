using DevTools;

namespace Api.AuthInternal;

enum AuthProvider {
  Email,
  Google,
  GitHub,
  Microsoft,
  SAML,
}

model User {
  id: string;
  email: string;
  name: string;
  image?: string;
  emailVerified: boolean;
  createdAt: string;
  updatedAt: string;
}

model Account {
  id: string;
  userId: string;
  provider: AuthProvider;
  providerAccountId: string;
  passwordHash?: string;
  accessToken?: string;
  oauthRefreshToken?: string;
  accessTokenExpiresAt?: string;
  scope?: string;
  idToken?: string;
  createdAt: string;
  updatedAt: string;
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

  /** BetterAuth session token — auth-service uses this to call GetToken and returns it as refreshToken to the client. */
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

model GetTokenRequest {
  /** BetterAuth session token — used to authenticate with BetterAuth's jwt() plugin to get a signed JWT. */
  sessionToken: string;
}

model GetTokenResponse {
  /** JWT access token signed by BetterAuth JWKS. */
  accessToken: string;
}

op GetToken(...GetTokenRequest): GetTokenResponse;

// -- OAuth RPCs --

model GetOAuthUrlRequest {
  provider: AuthProvider;
  callbackUrl: string;
  state?: string;
}

model GetOAuthUrlResponse {
  url: string;
  state: string;
}

op GetOAuthUrl(...GetOAuthUrlRequest): GetOAuthUrlResponse;

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

model GetAccountsByUserIdRequest {
  userId: string;
}

model GetAccountsByUserIdResponse {
  accounts: Account[];
  hasPassword: boolean;
}

op GetAccountsByUserId(...GetAccountsByUserIdRequest): GetAccountsByUserIdResponse;

model CreateAccountRequest {
  userId: string;
  provider: AuthProvider;
  providerAccountId: string;
  password?: string;
  accessToken?: string;
  oauthRefreshToken?: string;
  accessTokenExpiresAt?: string;
  scope?: string;
  idToken?: string;
}

model CreateAccountResponse {
  account: Account;
}

op CreateAccount(...CreateAccountRequest): CreateAccountResponse;
