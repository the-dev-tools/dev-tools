import "../core";
import "../../dist/src/ai-tools";

namespace DevTools.AITools {
  enum ToolCategory {
    Mutation,
    Exploration,
    Execution,
  }

  model AIToolOptions {
    category: ToolCategory;
    title?: string;
  }

  extern dec aiTool(target: Reflection.Model, options: valueof AIToolOptions);
}
