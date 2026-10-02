# Classifiers

A classifier is the part of lintuition that reads text. Each linter asks it one or two short
questions about a message, a comment or a test name, and Go code decides what to do with the answer.
The classifier is pluggable: pick one in `.lintuition.yml`, or write your own.

| classifier | |
|---|---|
| `fake` | deterministic scripted answers, local; for tests and offline runs |
| `jev` | [TypeSafe](https://typesafe.ai) System One models (Jev); remote, reads the API key from `TYPESAFE_API_KEY` |
| `openai` | OpenAI-compatible chat completions with log probabilities: OpenAI, or a local Ollama, llama.cpp, vLLM, LM Studio; local when the server is |
| `claude-code` | Claude Code (`claude -p`) under your login, with a fixed harness and a JSON schema; self-reported probabilities |
| `codex` | Codex CLI (`codex exec`) under your login, with a fixed harness and a JSON schema; self-reported probabilities |

```yaml
semantic:
  classifier: jev
  classifiers:
    jev:
      model: jev-1.13.0      # pin a version; jev-latest moves
```

Candidates are asked independently: putting neighbours in one request was measured to change
answers. How many requests a run makes depends on the backend, the number of questions, votes and
retries. The HTTP backends (`jev`, `openai`) retry on 429 (honouring `Retry-After`) and 5xx and are
rate-limited; the agent backends run one process per question. Costs are estimates at the
adapter's price assumption, not a bill. Errors never carry the request body or the key.

## Money

`semantic.budget.max-cost-usd` limits what one run may spend; `lintuition eval --max-cost-usd`
limits all its runs together. The limit counts spending the backend has already reported: once that
reaches it, no new question goes out, requests already in flight finish (so the total can end up
above the limit), and every finding made so far is still reported. The run counts as
incomplete (exit 2) and the summary says how many candidates were not asked. Answers already
received are cached for 168 hours by default, so a rerun soon after with a higher limit asks mostly
what is left.

Every remote backend can be capped: `jev` and `claude-code` know their cost, `codex` and remote
`openai` need `price-per-mtok`, and refuse a cap without it instead of ignoring it. A paid backend
without a cap gets a warning. A local model needs no cap.

To see exactly what would be sent, without sending it:

```sh
lintuition run --dry-run --preview requests.jsonl ./...
```

## How the backends compare

Measured with `lintuition eval` on 2026-10-01, on the twins this repository had then: six marked
defects from three linters. It shows how the backends behave, not how precise a linter is on real
code.

| classifier | model | defects caught | false alarms | requests | time | cost |
|---|---|---|---|---|---|---|
| `jev` | jev-latest | 6/6 in each of 3 runs | 0 | 48 (3 runs) | about 1 s a run | ~$0.0009 for 3 runs |
| `claude-code` | haiku | 6/6 | 0 | 16 | 66 s | ~$0.13 (Claude Code's own figure) |
| `codex` | default, reasoning-effort low | 6/6 | 0 | 16 | 39 s | subscription; ~14k input tokens a run |
| `openai` (Ollama, local) | qwen2.5:7b | 1/6 | 0 | 16 | 4 s | local |
| `openai` (Ollama, local) | qwen2.5:3b | 0/6 | 0 | 16 | 2 s | local |

On the [showcase](../examples/showcase) (all 16 linters, 17 marked findings), `jev-latest` caught
every finding in each of 3 runs with no unmarked findings, at 105 requests and ~$0.0019 for the
three runs (2026-10-02).

The small local models mostly answer "the text does not let you tell" or read "config saved to
disk" as an operation that is starting; the rules then abstain or stay quiet rather than report
noise. Thresholds tuned for one backend do not carry over to another. Run `lintuition eval` on your
own examples before trusting a new backend.

## A general LLM

`openai` turns a chat model into a classifier: each option gets a letter, the model is asked for
one letter, and the answer's probabilities are read from the log probabilities of that first token.
The probabilities come from the model's own token distribution, not from a number it writes.

```yaml
semantic:
  classifier: openai
  local-only: true                       # refuse anything that would leave the machine
  classifiers:
    openai:
      base-url: http://127.0.0.1:11434/v1 # Ollama
      model: qwen2.5:7b
```

## A coding agent

`claude-code` and `codex` run the agent's command line once per question, non-interactively, with
a fixed harness prompt (`agentcli.Harness`) and a JSON schema the answer must follow: the label of
one option and a probability for every option. An answer that is not one of the options, whose
probabilities do not add up to 1, or that is not its own most probable option fails the request.

```yaml
semantic:
  classifier: claude-code   # or codex
  classifiers:
    claude-code:
      model: haiku
      max-parallel: 4
    codex:
      reasoning-effort: low
      accept-agent-tools: true   # required, see below
      price-per-mtok: 1.25       # needed for a money cap
```

These runs leave the machine under the CLI's login. Claude Code runs with no tools, no MCP servers,
no settings and no saved session. Codex runs with a fresh `CODEX_HOME` that holds only a link to its
login (no user config, AGENTS.md, MCP servers, plugins or memories), with every optional tool
feature off, in its read-only sandbox. The Codex CLI still keeps a command tool that cannot be
switched off, so a model swayed by text in the payload could read files you can read; `codex`
refuses to run until `accept-agent-tools: true` says you accept that. Both start in an empty
directory.

Their probabilities are the model's own statement, not token probabilities: they are recorded as
self-reported, and a threshold tuned on another backend does not carry over. Every question is a
full agent run, so they are slower and dearer than a classifier API; they suit evaluation and small
projects better than a large CI run.

## Your own

More backends plug in behind the same `sdk.Classifier` interface; see [plugins](plugins.md) and the
`keywords` example in [examples/plugins](../examples/plugins).
