using DevTools;

namespace Api.Auth;

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

model LinkedAccount {
  provider: AuthProvider;
  providerAccountId: string;
  linkedAt: string;
}

model GetLinkedAccountsResponse {
  accounts: LinkedAccount[];
  hasPassword: boolean;
}

op GetLinkedAccounts(): GetLinkedAccountsResponse;

model AuthResponseFields {
  user: User;
  accessToken: string;
  refreshToken: string;
}

model SignUpRequest {
  email: string;
  password: string;
  name: string;
}

model SignUpResponse {
  ...AuthResponseFields;
}

@unauthenticated
op SignUp(...SignUpRequest): SignUpResponse;

model SignInRequest {
  email: string;
  password: string;
}

model SignInResponse {
  ...AuthResponseFields;
}

@unauthenticated
op SignIn(...SignInRequest): SignInResponse;

model SignOutRequest {
  refreshToken?: string;
}

model SignOutResponse {
  success: boolean;
}

op SignOut(...SignOutRequest): SignOutResponse;

model GetOAuthUrlRequest {
  provider: AuthProvider;
  callbackUrl: string;
  state?: string;
}

model GetOAuthUrlResponse {
  url: string;
  state: string;
}

@unauthenticated
op GetOAuthUrl(...GetOAuthUrlRequest): GetOAuthUrlResponse;

model HandleOAuthCallbackRequest {
  provider: AuthProvider;
  code: string;
  state?: string;
}

model HandleOAuthCallbackResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  isNewUser: boolean;
}

@unauthenticated
op HandleOAuthCallback(...HandleOAuthCallbackRequest): HandleOAuthCallbackResponse;

model RefreshTokenRequest {
  refreshToken: string;
}

model RefreshTokenResponse {
  accessToken: string;
  refreshToken: string;
}

@unauthenticated
op RefreshToken(...RefreshTokenRequest): RefreshTokenResponse;

model GetMeResponse {
  user: User;
}

op GetMe(): GetMeResponse;

model GetSSOUrlRequest {
  organizationId: string;
  callbackUrl: string;
}

model GetSSOUrlResponse {
  url: string;
  state: string;
}

@unauthenticated
op GetSSOUrl(...GetSSOUrlRequest): GetSSOUrlResponse;

model HandleSSOCallbackRequest {
  code: string;
  state?: string;
}

model HandleSSOCallbackResponse {
  ...AuthResponseFields;
  isNewUser: boolean;
}

@unauthenticated
op HandleSSOCallback(...HandleSSOCallbackRequest): HandleSSOCallbackResponse;
