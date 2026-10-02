# lintuition

[![CI](https://github.com/ssgreg/lintuition/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ssgreg/lintuition/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ssgreg/lintuition.svg)](https://pkg.go.dev/github.com/ssgreg/lintuition)
[![Release](https://img.shields.io/github/v/release/ssgreg/lintuition)](https://github.com/ssgreg/lintuition/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**[System One](https://en.wikipedia.org/wiki/Thinking,_Fast_and_Slow) linters for your Go code.**

lintuition finds places where code says one thing and does another: log messages, test names,
comments or metric Help that contradict the surrounding code.

It runs built-in linters and lets you add your own. You choose a classifier to interpret the wording;
each linter checks its answers against facts extracted by Go analysis.

Our quickstart uses [Jev](https://typesafe.ai). TypeSafe says it "returns typed decisions with
calibrated probabilities" and lists a price of $42 per billion input tokens. In our
[showcase measurement](docs/classifiers.md#how-the-backends-compare), Jev caught all 17 marked
findings in each of three runs at an estimated $0.0006 a run.

LLMs work too: an OpenAI-compatible API with logprobs support (including local models through
Ollama), or Claude Code and Codex. See [classifiers](docs/classifiers.md).

This function logs success before it knows whether the save worked:

```go
log.Print("config saved to disk")
if err := store.SaveConfig(cfg); err != nil {
	return err
}
```

```text
logs.go:15:2: success logged before SaveConfig has returned: "config saved to disk" (premature-success)
```

The compiler is fine with it, and so is golangci-lint.

A unit that does not match the value:

```go
return fmt.Sprintf("compaction took %.0f ms", d.Seconds())
// text says milliseconds, the value is in seconds
```

A test whose name and assertion disagree:

```go
func TestUnquoteRejectsUnbalancedQuote(t *testing.T) {
	_, err := Unquote(`"abc`)
	if err != nil {
		t.Fatal(err)
	}
}
// test name expects an error from Unquote, but the test fails when Unquote returns one
```

[More examples](#more-examples) below, and [the full list of linters](#linters).

## What it asks, sends and costs

- **Go code decides, not the model.** Typed analysis finds the place and the facts: this log call
  runs before `SaveConfig`, this `want` is compared with the result of `Expired`. The model answers
  one or two fixed questions about one piece of text ("does this message claim that SaveConfig has
  already completed?"), and Go code compares the answer with the facts.
- **You can see what leaves your machine.** The built-in linters send selected text (a message, a
  comment, a test name) and facts named by identifiers, not source files and not the values your
  code logs. `lintuition run --dry-run --preview requests.jsonl ./...` writes every request without
  sending any. With a local model and `semantic.local-only: true`, nothing leaves.
- **Unsure means quiet.** Below a linter's confidence threshold it stays silent whichever way the
  answer leans. Code it cannot read, like a message built at run time, is counted as unsupported in
  the summary instead of being guessed at.
- **Measured cost, with a budget.** On the showcase, Jev caught all 17 marked findings in each of 3
  runs for about $0.0006 a run ([measured 2026-10-02](docs/classifiers.md#how-the-backends-compare)).
  On an [earlier six-case suite](docs/classifiers.md#how-the-backends-compare), both Jev and Claude
  Code haiku caught all six defects. Jev took about a second at an estimated $0.0003 a run; Claude
  Code haiku took 66 seconds and reported about $0.13. `semantic.budget.max-cost-usd` stops new requests once reported spending reaches the limit;
  requests already in flight can take the total above it, and everything found so far is
  still reported. Valid cached answers are reused on the next run.

Findings are review hints with their evidence, not proofs.

## Try it

```sh
go install github.com/ssgreg/lintuition/cmd/lintuition@latest
```

Or download a binary for Linux, macOS or Windows from the
[releases page](https://github.com/ssgreg/lintuition/releases).

See the output without a model, on the showcase with scripted answers:

```sh
git clone https://github.com/ssgreg/lintuition
cd lintuition/examples/showcase
lintuition run ./...
```

On your own code with [Jev](https://typesafe.ai), from your project's root:

```sh
curl -fsSLo .lintuition.yml https://raw.githubusercontent.com/ssgreg/lintuition/main/.lintuition.quickstart.yml
export TYPESAFE_API_KEY=...
lintuition run --dry-run --preview requests.jsonl ./...   # what would be sent; nothing is
lintuition run ./...
```

The quickstart config exits 0 even when it finds something, so a first run in CI does not fail the
build; remove `run.issues-exit-code: 0` to make findings fail it.

No Jev key? Set up another classifier from [classifiers](docs/classifiers.md).

> Status: v0.x. Until v1.0, a minor release may change the config, the command line and the `sdk`
> API; [CHANGELOG.md](CHANGELOG.md) says what changed.

## More examples

A table row copied without flipping `want`:

```go
{name: "expired when the deadline has passed", passed: true, want: true},
{name: "not expired before the deadline", passed: false, want: true},
// case "not expired before the deadline" reads as want false, but the table sets true
```

A routine event logged as an error, and a real loss logged as info:

```go
slog.Error("cache miss, loading from the database", "key", key)
// routine event logged at error level

slog.Info("events for the last hour are lost, the write to disk was refused")
// unintended loss logged at info level
```

A comment that slid onto the wrong constant:

```go
const (
	// waiting for a free worker
	PhaseQueued Phase = iota
	// every block has been copied and verified
	PhaseCopying
	PhaseVerified
)
// comment describes PhaseVerified, not PhaseCopying
```

Every example on this page is real code in [examples/showcase](examples/showcase), together with one
for each of the other linters.

## Linters

How each one reads your code, what it sends and how it decides, with an example:
[docs/linters.md](docs/linters.md).

| | linter | catches |
|---|---|---|
| logs | `premature-success` | a log line reports success before the call that can still fail |
| | `log-sensitive-field` | a structured log field that logs a secret (token, password, key material) as is |
| | `log-key-value-role` | a log key that names a different quantity than the variable logged under it |
| | `normal-event-at-error` | an expected routine event (cache miss, retry scheduled) logged at error level |
| | `severe-event-understated` | an unintended loss of data or work, or an outage, logged at debug or info |
| | `destructive-remediation` | a message that advises deleting, wiping or reinstalling without saying what is lost |
| errors | `sentinel-name-vs-text` | a sentinel error whose name and message describe different conditions |
| | `error-needs-type` | policy, off by default: a branchable condition (not found, already exists) returned as a plain string error |
| units, comments | `human-unit-contradiction` | a printf message names a different unit than the duration it prints |
| | `enum-comment-shift` | a comment in a const block that describes a neighbouring constant |
| | `doc-vs-signature` | a doc that promises a result or an error the function's signature does not have |
| tests | `test-name-vs-assertion` | a test named for a failure that asserts success, or the reverse |
| | `table-case-vs-expectation` | a table case whose name says the opposite of its boolean `want` |
| | `doc-vs-table` | a table case whose `want` contradicts the tested function's doc |
| metrics | `metric-type-vs-help` | Prometheus Help that describes a different kind of value than the metric records |
| nolint | `suppression-rationale` | a `//nolint` reason that explains something other than what the linter reports |

## Works like golangci-lint

lintuition runs next to golangci-lint, not instead of it. Its command line, config and output follow
golangci-lint v2 where it supports the same thing.

```sh
lintuition run ./...                          # file:line:col: message (linter)
lintuition run --output.sarif.path out.sarif ./...
lintuition linters                            # what the config enables
lintuition config verify                      # unknown keys, linters and settings are errors
```

- **Config:** `.lintuition.yml`, found from the working directory upwards, in the golangci-lint v2
  shape plus a `semantic` section for the classifier. Every key with its default is in
  [.lintuition.reference.yml](.lintuition.reference.yml).
- **Formats:** `text`, `json`, `sarif`, `checkstyle`, `code-climate`, `junit-xml` and
  `github-actions`.
- **Suppressing:** `//nolint:premature-success // why it is fine here`, with the same scopes as
  golangci-lint. The directive must name the linter and give a reason; a bare `//nolint` does not
  suppress.
- **Exit codes:** findings exit `1` by default, or what `run.issues-exit-code` says; a clean run
  exits `0`. A run that is incomplete (a package did not load, a request failed, a budget was
  reached) exits `2`, whatever the findings.

This repository lints itself on every push to main: [the workflow](.github/workflows/self-lint.yml)
sends findings to GitHub code scanning and keeps the answer cache between runs.

## What leaves the machine

`semantic.payload` decides what a request may carry:

| policy | sends |
|---|---|
| `facts` | facts computed by code: identifiers, types, kinds, counts |
| `prose` (default) | facts plus the text a person wrote: comments, metric Help, log messages, test names |
| `source` | also Go source text; no built-in linter asks for it |

A candidate that needs more than the policy allows is skipped whole and counted, never sent stripped.
`semantic.local-only: true` refuses any backend that sends requests off the machine.

## More

- [How the linters work](docs/linters.md), one section per linter
- [Classifiers](docs/classifiers.md): Jev, a local or remote LLM, Claude Code, Codex, budgets,
  and how they compare
- [Plugins](docs/plugins.md): your own linters and classifiers, built into a custom binary

## License

MIT
