# lintuition

**System One for your Go code.**
Checks whether comments, logs and test descriptions match what your code does.

```text
metrics.go:10:30: counter Help describes a current value, not a running total: "Disk I/O utilization." (metric-type-vs-help)
	ioSeconds = prom.NewCounter(prom.CounterOpts{Name: "io_seconds_total", Help: ioHelp})
	                            ^
premature/premature.go:13:2: success logged before SaveConfig has returned: "config saved to disk" (premature-success)
	slog.Info("config saved to disk")
	^
```

> Status: early development (pre-v0.1). The interfaces and the config may change.

## How it works

A classifier reads the words; Go code checks the logic.

1. Each linter's `go/analysis` analyzer finds candidates in typed Go code and extracts facts: the
   metric's type, the Help text, the function a log line runs before.
2. The linter asks a classifier narrow typed questions about the person-written text: a *choice*, a
   *yes/no* or a *score*. "What kind of value does this Help describe: a running total, a current
   value or a distribution?"
3. Go code compares the answer with the extracted facts and decides. The classifier never decides
   whether something is a finding, and never sees the syntax tree.

Findings are review hints with their evidence, not proofs.

## Quick start

```sh
go install github.com/ssgreg/lintuition/cmd/lintuition@latest
```

The offline sample lives in this repository:

```sh
git clone https://github.com/ssgreg/lintuition && cd lintuition/examples/sample
lintuition run ./...   # runs offline with the scripted fake classifier
```

On your code, with Jev (`.lintuition.quickstart.yml` is in the repository and in every release
archive):

```sh
cp .lintuition.quickstart.yml your/project/.lintuition.yml
cd your/project
export TYPESAFE_API_KEY=...
lintuition run --dry-run --preview requests.jsonl ./...   # what would be sent, and how much
lintuition run ./...
```

Release binaries for Linux, macOS and Windows are on the
[releases page](https://github.com/ssgreg/lintuition/releases).

The command line, config and output follow golangci-lint v2 where lintuition supports the same thing:

```sh
lintuition run ./...                  # text: file:line:col: message (linter)
lintuition run --output.json.path stdout ./...
lintuition run --dry-run ./...        # plan the requests, send none
lintuition linters                    # what the config enables
lintuition classifiers                # available backends
lintuition config verify              # strict: unknown keys, linters and settings are errors
```

Formats: `text`, `json`, `sarif`, `checkstyle`, `code-climate`, `junit-xml` and, as an extension
(golangci-lint v2 dropped it), `github-actions`. Each has `--output.<format>.path`; each needs its own
destination.

### Suppressing a finding

```go
x := f() //nolint:premature-success // f is a pure lookup and cannot fail
```

As in golangci-lint, a directive at the end of a line covers the line; one on its own line above a
declaration or statement covers it; one above the package clause covers the file. Stricter than
golangci-lint by default: the directive must name the linter and give a reason after `//`. A bare
`//nolint`, `//nolint:all` or a directive without a reason does not suppress. A suppressed candidate
is never sent to the classifier.

Exit codes: `0` clean, `1` issues found (`run.issues-exit-code`), `2` the run is incomplete or failed:
a package did not load, a request failed, or a budget was reached. An incomplete run never exits 0.

## Configuration

`.lintuition.yml`, looked up from the working directory upwards. Every supported key with its default
is in [.lintuition.reference.yml](.lintuition.reference.yml).

```yaml
version: "2"
linters:
  default: standard
semantic:
  classifier: fake
  payload: prose      # facts | prose | source
```

### What leaves the machine

`semantic.payload` decides what a request may carry:

| policy | sends |
|---|---|
| `facts` | structural facts computed by code: types, kinds, counts |
| `prose` (default) | facts plus person-written text: comments, metric Help, log messages, test names |
| `source` | also Go source text |

A candidate that needs more than the policy allows is skipped whole and counted, never sent stripped.
`semantic.local-only: true` refuses any backend that sends requests off the machine. Under `prose`,
person-written text does leave the machine when the backend is remote.

## Linters

| linter | checks |
|---|---|
| `metric-type-vs-help` | Prometheus metric Help that describes a different kind of value than the metric type records |
| `premature-success` | a log line reports success before the call that can still fail |
| `table-case-vs-expectation` | a table test case whose name says the opposite of its boolean expectation |

Each linter has defect / fixed twins in [testdata/twins](testdata/twins), checked by
[`linttest`](linttest) with `// want` comments, like `analysistest`.

`lintuition eval` runs the same twins against a real classifier, several fresh times with the cache
off, and reports how often each marked case was caught and every finding no mark accounts for:

```text
$ lintuition eval -c jev.yml --runs 3 testdata/twins
6 marked cases x 3 runs: caught in every run 6, in some 0, in none 0; 0 unaccounted finding(s);
7 abstentions; 48 requests, ~$0.000857
```

That is the twins of this repository with `jev-latest` on 2026-10-01. Twins are small and written to
exercise the rules; they check that the pieces work together, not how precise a linter is on real
code.

Facts are read through `go/types`: an error result is found by its type, not by the variable's
name; a log call is a level-named call of a logger package that returns nothing (so `zap.Error(err)`
is a field, not a log line); a table row is bound to the function called in the loop over that
table. A shape the analyzer sees but cannot read, such as a message built at run time, is counted
as unsupported rather than silently dropped.

## Classifiers

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

Every candidate is its own request: neighbours in one request were measured to change answers.
Requests are retried on 429 (honouring `Retry-After`) and 5xx, rate-limited, and capped by
`semantic.budget`. Costs are estimates at the adapter's price assumption, not a bill. Errors never
carry the request body or the key.

### Money

`semantic.budget.max-cost-usd` caps what one run may spend; `lintuition eval --max-cost-usd` caps
all its runs together. When the cap is reached, no new question goes out, requests already in
flight finish, and every finding made so far is still reported; the run counts as incomplete
(exit 2) and the summary says how many candidates were not asked. Answers already received are
cached, so a rerun with a higher cap pays only for the rest.

Every remote backend can be capped: `jev` and `claude-code` know their cost, `codex` and remote
`openai` need `price-per-mtok`, and refuse a cap without it instead of ignoring it. A paid backend
without a cap gets a warning. A local model needs no cap.

To see exactly what would be sent, without sending it:

```sh
lintuition run --dry-run --preview requests.jsonl ./...
```

### A general LLM as the classifier

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

How good the answers are depends on the model, and thresholds tuned for one backend do not carry
over. Measured on this repository's twins, 2026-10-01:

| classifier | model | defects caught | false alarms | requests | time | cost |
|---|---|---|---|---|---|---|
| `jev` | jev-latest | 6/6 in each of 3 runs | 0 | 48 (3 runs) | about 1 s a run | ~$0.0009 for 3 runs |
| `claude-code` | haiku | 6/6 | 0 | 16 | 66 s | ~$0.13 (Claude Code's own figure) |
| `codex` | default, reasoning-effort low | 6/6 | 0 | 16 | 39 s | subscription; ~14k input tokens a run |
| `openai` (Ollama, local) | qwen2.5:7b | 1/6 | 0 | 16 | 4 s | local |
| `openai` (Ollama, local) | qwen2.5:3b | 0/6 | 0 | 16 | 2 s | local |

The small local models mostly answer "the text does not let you tell" or read "config saved to
disk" as an operation that is starting; the rules then abstain or stay quiet rather than report
noise. Run `lintuition eval` on your twins before trusting another backend.

### A coding agent as the classifier

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
directory. Their probabilities are the model's own statement, not token
probabilities: they are recorded as self-reported, and a threshold tuned on another backend does
not carry over. Every question is a full agent run, so they are slower and dearer than a
classifier API; they suit evaluation and small projects better than a large CI run.

More backends can plug in behind the same `sdk.Classifier` interface.

## Plugins

Linters and classifiers both plug in, in the spirit of golangci-lint's module plugin system: they
implement the small contracts in [`sdk`](sdk), register from `init`, and are compiled into a custom
binary. Nothing is loaded at run time.

```yaml
# .custom-lintuition.yml
version: v0.1.0            # or path: ../lintuition for a local checkout
name: custom-lintuition    # default
destination: ./bin         # default .
plugins:
  - module: example.com/lintuition-todo-owner
    version: v1.0.0
  - module: example.com/lintuition-keywords
    path: ./plugins/keywords
```

```sh
lintuition custom          # builds ./bin/custom-lintuition
./bin/custom-lintuition version   # lists the plugins compiled in
```

Two example plugins live in their own modules under [examples/plugins](examples/plugins): a linter,
`todo-owner`, and a local classifier, `keywords`. They import only `sdk`.

Plugins are trusted code: they run in-process with your rights and see every candidate, so the
payload policy binds the built-in linters, not a plugin that chooses to ignore it. The `sdk` API is
pre-v1 and may change in minor releases until v1.0.0.

## License

MIT
