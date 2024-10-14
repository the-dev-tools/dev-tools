using TypeSpec.Protobuf;

@package({
  name: "auth.v1",
  options: {
    go_package: "dev-tools-services/gen/auth/v1;authv1",
  },
})
namespace API.Auth;

model AuthTokens {
  @field(1) accessToken: string;
  @field(2) refreshToken: string;
}

@Protobuf.service
interface AuthService {
  /** https://magic.link/docs/authentication/features/decentralized-id */
  AuthMagicLink(@field(1) didToken: string): AuthTokens;
  AuthRefresh(@field(1) refreshToken: string): AuthTokens;
}
