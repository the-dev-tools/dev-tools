import "@typespec/protobuf";

import "../lib";

using TypeSpec.Protobuf;

@package({
  name: "auth.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/auth/v1;authv1",
  },
})
namespace API.Auth;

@autoFields
model AuthTokens {
  accessToken: string;
  refreshToken: string;
}

@autoFields
model AuthMagicLinkRequest {
  /** https://magic.link/docs/authentication/features/decentralized-id */
  didToken: string;
}
model AuthMagicLinkResponse is AuthTokens;

@autoFields
model AuthRefreshRequest {
  refreshToken: string;
}
model AuthRefreshResponse is AuthTokens;

@Protobuf.service
interface AuthService {
  AuthMagicLink(...AuthMagicLinkRequest): AuthMagicLinkResponse;
  AuthRefresh(...AuthRefreshRequest): AuthRefreshResponse;
}
