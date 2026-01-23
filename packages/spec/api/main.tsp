import "@the-dev-tools/spec-lib/core";
import "@the-dev-tools/spec-lib/protobuf";
import "@the-dev-tools/spec-lib/tanstack-db";
import "@the-dev-tools/spec-lib/ai-tools";

import "./environment.tsp";
import "./export.tsp";
import "./file-system.tsp";
import "./flow.tsp";
import "./health.tsp";
import "./http.tsp";
import "./import.tsp";
import "./log.tsp";
import "./node-js-executor.tsp";
import "./reference.tsp";
import "./workspace.tsp";

using DevTools;

alias Id = bytes;

model CommonTableFields<TParent extends Reflection.Model> {
  ...Keys<TParent, #{ primary: KeyAs.Foreign, foreign: KeyAs.Omit }>;
  @doc("Variable name (used to reference it in expressions)") key: string;
  @doc("Whether the variable is active") enabled: boolean;
  @doc("Variable value") value: string;
  @doc("Description of what the variable is for") description: string;
  @doc("Display order") order: float32;
}

@DevTools.project
namespace Api {

}
