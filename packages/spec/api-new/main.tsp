import "@the-dev-tools/spec-lib/core";
import "@the-dev-tools/spec-lib/protobuf";
import "@the-dev-tools/spec-lib/tanstack-db";

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

alias Id = bytes;

model CommonTableFields {
  key: string;
  enabled: boolean;
  value: string;
  description: string;
  order: float32;
}

@DevTools.project
namespace Api {

}
