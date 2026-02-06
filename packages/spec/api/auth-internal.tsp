using DevTools;

namespace Api.AuthInternal;

enum AuthProvider {
  Email,
  Google,
  GitHub,
  Microsoft,
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

model RefreshToken {
  id: string;
  userId: string;
  token: string;
  expiresAt: string;
  ipAddress?: string;
  userAgent?: string;
  createdAt: string;
}

model JwtClaims {
  sub: string;
  email: string;
  name: string;
  iat: int64;
  exp: int64;
}

model CreateUserRequest {
  email: string;
  name: string;
  image?: string;
}

model CreateUserResponse {
  user: User;
}

op CreateUser(...CreateUserRequest): CreateUserResponse;

model CreateUserWithPasswordRequest {
  email: string;
  name: string;
  password: string;
}

model CreateUserWithPasswordResponse {
  user: User;
  account: Account;
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
}

op VerifyCredentials(...VerifyCredentialsRequest): VerifyCredentialsResponse;

model GetUserRequest {
  userId: string;
}

model GetUserResponse {
  user?: User;
}

op GetUser(...GetUserRequest): GetUserResponse;

model CreateTokensRequest {
  userId: string;
  email: string;
  name: string;
  ipAddress?: string;
  userAgent?: string;
}

model CreateTokensResponse {
  accessToken: string;
  refreshToken: string;
}

op CreateTokens(...CreateTokensRequest): CreateTokensResponse;

model RefreshTokensRequest {
  refreshToken: string;
}

model RefreshTokensResponse {
  accessToken: string;
  refreshToken: string;
  user: User;
}

op RefreshTokens(...RefreshTokensRequest): RefreshTokensResponse;

model RevokeRefreshTokenRequest {
  refreshToken: string;
}

model RevokeRefreshTokenResponse {
  success: boolean;
}

op RevokeRefreshToken(...RevokeRefreshTokenRequest): RevokeRefreshTokenResponse;

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
  ipAddress?: string;
  userAgent?: string;
}

model ExchangeOAuthCodeResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  isNewUser: boolean;
}

op ExchangeOAuthCode(...ExchangeOAuthCodeRequest): ExchangeOAuthCodeResponse;

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
