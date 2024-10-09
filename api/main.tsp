import "@typespec/rest";
import "@typespec/openapi";

import "../extensions";

import "./variable.tsp";
import "./workspace.tsp";

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
