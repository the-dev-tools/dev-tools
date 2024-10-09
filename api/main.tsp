import "@typespec/rest";
import "@typespec/openapi";

import "../extensions";

using TypeSpec.Http;

@service({
  title: "API",
})
@useAuth(BearerAuth)
namespace API;

@error
model Error {
  code: int32;
  message: string;
}
