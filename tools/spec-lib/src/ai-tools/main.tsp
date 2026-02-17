import "../core";
import "../../dist/src/ai-tools";

namespace DevTools.AITools {
  enum ToolCategory {
    Mutation,
    Execution,
  }

  model AIToolOptions {
    category: ToolCategory;
    title?: string;
  }

  extern dec aiTool(target: Reflection.Model, options: valueof AIToolOptions);

  enum CrudOperation {
    Insert,
    Update,
    Delete,
  }

  model IncludeFromModel {
    fromModel: string;
    fields: string[];
  }

  model MutationToolOptions {
    operation: CrudOperation;
    title?: string;
    name?: string;
    description?: string;
    parent?: string;
    exclude?: string[];
    include?: IncludeFromModel[];
  }

  extern dec mutationTool(target: Reflection.Model, ...tools: valueof MutationToolOptions[]);
}
