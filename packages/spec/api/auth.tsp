namespace Api.Auth;

enum AuthProvider {
  ...Internal.Auth.AuthProvider,
}

model AuthResponseFields {
  userId: Id;
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

op SignUp(...SignUpRequest): SignUpResponse;

model SignInRequest {
  email: string;
  password: string;
}

model SignInResponse {
  ...AuthResponseFields;
}

op SignIn(...SignInRequest): SignInResponse;

model SignOutRequest {
  refreshToken?: string;
}

model SignOutResponse {}

op SignOut(...SignOutRequest): SignOutResponse;

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

model HandleOAuthCallbackRequest {
  provider: AuthProvider;
  code: string;
  state?: string;
}

model HandleOAuthCallbackResponse {
  ...AuthResponseFields;
  isNewUser: boolean;
}

op HandleOAuthCallback(...HandleOAuthCallbackRequest): HandleOAuthCallbackResponse;

model RefreshTokenRequest {
  refreshToken: string;
}

model RefreshTokenResponse {
  accessToken: string;
  refreshToken: string;
}

op RefreshToken(...RefreshTokenRequest): RefreshTokenResponse;
