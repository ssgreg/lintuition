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
cd examples/sample && lintuition run ./...   # runs offline with the scripted fake classifier
```

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

To see exactly what would be sent, without sending it:

```sh
lintuition run --dry-run --preview requests.jsonl ./...
```

More backends (a hosted LLM API, a local model) can plug in behind the same `sdk.Classifier`
interface.

## Plugins

Linters and classifiers implement the small contracts in [`sdk`](sdk) and register from `init`.
A custom binary imports `github.com/ssgreg/lintuition/builtin` plus the plugin packages and calls
`cli.Main`. A builder command is planned.

## License

MIT
