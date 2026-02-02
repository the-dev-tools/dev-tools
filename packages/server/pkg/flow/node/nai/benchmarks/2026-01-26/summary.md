# AI Node POC Benchmark Summary

**Date:** 2026-01-26
**Models Tested:** 3
**Total Test Runs:** 44

## Quick Overview

| Model | Best POC | Score | Success Rate | Avg Time |
|-------|----------|-------|--------------|----------|
| Claude Sonnet 4.5 | POC3-Discovery | 98.5 | 100% (6/6) | 1.69s |
| Gemini 2.5 Pro | POC7-ReAct | 83.9 | 100% (6/6) | 2.83s |
| GPT-5.2* | POC6-FewShot | 100.0 | 100% (1/1) | 2.39s |

*GPT-5.2 only ran Quick test (Simple scenario, 1 run)

---

## Cross-Model POC Rankings

### Best POC by Model

| POC | Claude 4.5 | Gemini 2.5 | GPT-5.2 |
|-----|------------|------------|---------|
| POC1-Introspect | #7 (93.0) | #6 (61.1) | #5 (82.3) |
| POC2-UserDesc | #5 (94.7) | #3 (74.1) | #3 (97.5) |
| POC3-Discovery | **#1 (98.5)** | #2 (75.1) | - |
| POC4-TypedParam | #3 (96.7) | #4 (69.9) | #4 (97.5) |
| POC5-AutoChain | #4 (95.7) | #5 (63.4) | #2 (99.0) |
| POC6-FewShot | #2 (97.0) | #7 (59.4) | **#1 (100.0)** |
| POC7-ReAct | #6 (94.0) | **#1 (83.9)** | #6 (80.0) |

### Success Rate by Model

| POC | Claude 4.5 | Gemini 2.5 | GPT-5.2 |
|-----|------------|------------|---------|
| POC1-Introspect | 100% | 67% | 100% |
| POC2-UserDesc | 100% | 100% | 100% |
| POC3-Discovery | 100% | 100% | - |
| POC4-TypedParam | 100% | 100% | 100% |
| POC5-AutoChain | 100% | 83% | 100% |
| POC6-FewShot | 100% | 67% | 100% |
| POC7-ReAct | 100% | 100% | 100% |

---

## Detailed Results by Model

### Claude Sonnet 4.5 (`claude-sonnet-4-5-20250929`)

**Full Benchmark:** 3 scenarios, 2 runs each

| Rank | POC | Score | Success | Avg Calls | Avg Time |
|------|-----|-------|---------|-----------|----------|
| 1 | POC3-Discovery | 98.5 | 6/6 | 0.0 | 1.69s |
| 2 | POC6-FewShot | 97.0 | 6/6 | 0.0 | 2.11s |
| 3 | POC4-TypedParam | 96.7 | 6/6 | 0.0 | 2.22s |
| 4 | POC5-AutoChain | 95.7 | 6/6 | 0.0 | 2.50s |
| 5 | POC2-UserDesc | 94.7 | 6/6 | 0.0 | 2.78s |
| 6 | POC7-ReAct | 94.0 | 6/6 | 0.0 | 2.99s |
| 7 | POC1-Introspect | 93.0 | 6/6 | 0.0 | 3.27s |

**Key Finding:** All POCs achieved 100% success. POC3-Discovery fastest and highest scoring.

---

### Gemini 2.5 Pro (`gemini-2.5-pro`)

**Full Benchmark:** 3 scenarios, 2 runs each

| Rank | POC | Score | Success | Avg Calls | Avg Time |
|------|-----|-------|---------|-----------|----------|
| 1 | POC7-ReAct | 83.9 | 6/6 | 0.2 | 2.83s |
| 2 | POC3-Discovery | 75.1 | 6/6 | 1.7 | 3.23s |
| 3 | POC2-UserDesc | 74.1 | 6/6 | 2.3 | 2.57s |
| 4 | POC4-TypedParam | 69.9 | 6/6 | 2.7 | 2.86s |
| 5 | POC5-AutoChain | 63.4 | 5/6 | 2.5 | 3.06s |
| 6 | POC1-Introspect | 61.1 | 4/6 | 1.7 | 3.41s |
| 7 | POC6-FewShot | 59.4 | 4/6 | 2.3 | 2.97s |

**Key Finding:** Less reliable than Claude. POC6-FewShot scored lowest (failed Complex scenarios).

---

### GPT-5.2 (`gpt-5.2`)

**Quick Benchmark:** Simple scenario only, 1 run

| Rank | POC | Score | Success | Avg Calls | Avg Time |
|------|-----|-------|---------|-----------|----------|
| 1 | POC6-FewShot | 100.0 | 1/1 | 1.0 | 2.39s |
| 2 | POC5-AutoChain | 99.0 | 1/1 | 1.0 | 2.76s |
| 3 | POC2-UserDesc | 97.5 | 1/1 | 1.0 | 3.37s |
| 4 | POC4-TypedParam | 97.5 | 1/1 | 1.0 | 3.38s |
| 5 | POC1-Introspect | 82.3 | 1/1 | 1.0 | 9.38s |
| 6 | POC7-ReAct | 80.0 | 1/1 | 1.0 | 10.31s |

**Key Finding:** Limited data (Quick test only). Full benchmark needed for accurate comparison.

---

## POC Production Status

| POC | In Production | Notes |
|-----|---------------|-------|
| POC1-Introspect | Yes | `VariableIntrospector` interface |
| POC2-UserDesc | Yes | `DescribableNode` interface |
| POC3-Discovery | Yes | `discover_tools` function |
| POC4-TypedParam | Yes | `{{ ai('name', 'desc', 'type') }}` syntax |
| POC5-AutoChain | Yes | `SourceHint` field for chaining |
| POC6-FewShot | **No** | Test only - hardcoded examples |
| POC7-ReAct | **No** | Test only - reasoning prompts |

---

## Important: POC6 & POC7 Are NOT Realistic

**POC6 (Few-Shot)** and **POC7 (ReAct)** score well but are **not suitable for production**:

### Why POC6 Scores Are Misleading

- Test provides "perfect" step-by-step examples hardcoded in the prompt
- Example from test:
  ```
  ### Example 1: Fetching user data
  1. Call set_variable with key="ai_1.userId" and value="42"
  2. Call GetUser tool
  3. Call get_variable with key="GetUser.response.body.name"
  ```
- In real world, users would need to manually write these examples for **every flow**
- This defeats the purpose of AI automation
- The benchmark essentially "cheats" by telling AI exactly what to do

### Why POC7 Has Limitations

- Adds "think step by step" reasoning to prompts
- Helps accuracy but adds latency and token cost
- Not implemented in production code

### Production Recommendation

Use **POC3 (Discovery) + POC4 (TypedParam) + POC5 (AutoChain)**:
- Work automatically without manual examples
- POC3 scored 98.5 with Claude (nearly same as POC6's 97.0)
- No user configuration required
- Already implemented in production

---

## Scoring Methodology

| Weight | Metric | Description |
|--------|--------|-------------|
| 40% | Success Rate | Did the task complete correctly? |
| 30% | Efficiency | Fewer tool calls = better |
| 20% | Speed | Faster execution time |
| 10% | Reliability | Consistency across runs (std dev) |

---

## Conclusions

1. **Best Model:** Claude Sonnet 4.5 - highest scores, 100% reliability
2. **Best Production POC:** POC3-Discovery (98.5 with Claude)
3. **Avoid:** POC6/POC7 - high scores are misleading (test-only conditions)
4. **Gemini 2.5:** Less reliable, some POCs fail on Complex scenarios
5. **GPT-5.2:** Needs full benchmark for fair comparison
