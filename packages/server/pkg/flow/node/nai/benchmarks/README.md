# AI Node POC Benchmarks

This folder contains benchmark results comparing different approaches (POCs) for AI tool discovery and execution.

## POC Descriptions

| POC | Name | Approach |
|-----|------|----------|
| POC1 | Introspection | Auto-generated descriptions from `VariableIntrospector` interface |
| POC2 | User Description | Manual/custom description field on nodes |
| POC3 | Discovery Tool | On-demand `discover_tools` function |
| POC4 | Typed Parameters | `{{ ai('name', 'desc', 'type') }}` syntax |
| POC5 | Auto-Chaining | `{{ ai('name', 'desc', 'type', 'source') }}` with chain hints |
| POC6 | Few-Shot Examples | Example tool call sequences in prompt |
| POC7 | ReAct Pattern | Explicit reasoning before each action |
| POC8 | Dependency Graph | Visual tool execution order |

## Scoring Weights

- **Success Rate (40%)**: Did the task complete correctly?
- **Efficiency (30%)**: Fewer tool calls = better (less cost/latency)
- **Speed (20%)**: Faster execution time
- **Reliability (10%)**: Consistency across multiple runs

## Scenarios

- **Simple**: Single tool call (GetUser)
- **Medium**: 3-tool chain (GetUser → GetPosts → GetComments)
- **Complex**: Data pipeline (FetchData → TransformData → ValidateResult)

## Preferred Models

| Provider | Model | Score | Success | Notes |
|----------|-------|-------|---------|-------|
| Anthropic | `claude-sonnet-4-5-20250929` | 98.5 | 100% | ⭐ Best overall |
| Google | `gemini-2.5-pro` | 83.9 | Mixed | Good but less reliable |
| OpenAI | `gpt-5.2` | 100* | 100% | *Quick test only |

> **Note**: Model names must match the provider's API exactly.

## Running Benchmarks

### Environment Variables

| Provider | Required | Optional |
|----------|----------|----------|
| OpenAI | `OPENAI_API_KEY` | `OPENAI_MODEL`, `OPENAI_BASE_URL` |
| Anthropic | `ANTHROPIC_API_KEY` | `ANTHROPIC_MODEL`, `ANTHROPIC_BASE_URL` |
| Google | `GEMINI_API_KEY` | - |

### Examples

```bash
# Anthropic Claude Sonnet 4.5 (recommended)
RUN_AI_INTEGRATION_TESTS=true ANTHROPIC_API_KEY=sk-ant-... \
  ANTHROPIC_MODEL=claude-sonnet-4-5-20250929 \
  go test -tags ai_integration -v -run TestPOC_Benchmark_Full \
  ./packages/server/pkg/flow/node/nai

# Google Gemini 2.5 Pro
RUN_AI_INTEGRATION_TESTS=true GEMINI_API_KEY=... \
  GEMINI_MODEL=gemini-2.5-pro \
  go test -tags ai_integration -v -run TestPOC_Benchmark_Full \
  ./packages/server/pkg/flow/node/nai

# OpenAI GPT-5.2
RUN_AI_INTEGRATION_TESTS=true OPENAI_API_KEY=sk-... \
  OPENAI_MODEL=gpt-5.2 \
  go test -tags ai_integration -v -run TestPOC_Benchmark_Full \
  ./packages/server/pkg/flow/node/nai
```

## File Format

- `YYYY-MM-DD_provider-model.json` - Raw benchmark data
- `YYYY-MM-DD_provider-model.md` - Human-readable report
