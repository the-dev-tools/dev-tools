import "@the-dev-tools/spec-lib/core";
import "@the-dev-tools/spec-lib/protobuf";
import "@the-dev-tools/spec-lib/data-client";

import "./auth.tsp";
import "./collection-item/body.tsp";
import "./collection-item/endpoint.tsp";
import "./collection-item/example.tsp";
import "./collection-item/folder.tsp";
import "./collection-item/item.tsp";
import "./collection-item/request.tsp";
import "./collection-item/response.tsp";
import "./collection.tsp";
import "./condition.tsp";
import "./delta.tsp";
import "./environment.tsp";
import "./export.tsp";
import "./flow-item/edge.tsp";
import "./flow-item/node-execution.tsp";
import "./flow-item/node.tsp";
import "./flow-variable.tsp";
import "./flow.tsp";
import "./health.tsp";
import "./import.tsp";
import "./log.tsp";
import "./main.tsp";
import "./node-js-executor.tsp";
import "./reference.tsp";
import "./resource.tsp";
import "./tag.tsp";
import "./variable.tsp";
import "./workspace.tsp";

@DevTools.project
namespace API;

/** Prettier workaround to allow comments before first interface */
interface BlankInterface {}
