import "@the-dev-tools/spec-lib/core";
import "@the-dev-tools/spec-lib/protobuf";
import "@the-dev-tools/spec-lib/tanstack-db";

import "./credential.tsp";
import "./environment.tsp";
import "./export.tsp";
import "./file-system.tsp";
import "./flow.tsp";
import "./health.tsp";
import "./http.tsp";
import "./import.tsp";
import "./private/auth-adapter.tsp";
import "./private/node-js-executor.tsp";
import "./log.tsp";
import "./organization.tsp";
import "./reference.tsp";
import "./user.tsp";
import "./workspace.tsp";

using DevTools;

alias Id = bytes;

model CommonTableFields<TParent extends Reflection.Model> {
  ...Keys<TParent, #{ primary: KeyAs.Foreign, foreign: KeyAs.Omit }>;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
  order: float32;
}

@DevTools.project
namespace Api {

}
