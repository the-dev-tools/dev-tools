import "@typespec/protobuf";

using TypeSpec.Protobuf;

@package({
  name: "auth.v1",
  options: {
    go_package: "dev-tools-spec/dist/@buf/go/auth/v1;authv1",
  },
})
namespace API.Auth;

model AuthTokens {
  @field(1) accessToken: string;
  @field(2) refreshToken: string;
}

model AuthMagicLinkRequest {
  /** https://magic.link/docs/authentication/features/decentralized-id */
  @field(1) didToken: string;
}
model AuthMagicLinkResponse is AuthTokens;

model AuthRefreshRequest {
  @field(1) refreshToken: string;
}
model AuthRefreshResponse is AuthTokens;

@Protobuf.service
interface AuthService {
  AuthMagicLink(...AuthMagicLinkRequest): AuthMagicLinkResponse;
  AuthRefresh(...AuthRefreshRequest): AuthRefreshResponse;
}
