# Changelog

## Unreleased

- `openai`: `reasoning-effort`, so a model that thinks first (qwen3.x, gemma4 on Ollama) answers
  with its first token; a confident answer no longer fails on a confidence a rounding error above 1.
- New linter `doc-vs-signature`: a doc comment that promises a returned result or error the
  function's signature does not have.
- `destructive-remediation` (version 3) no longer reads debug and info logs, sends the log level
  and the function called right after a log call, has an answer for a message that names the
  program's own action, and counts its three non-destructive answers together. On 25 repositories
  its 35 findings were all a program announcing its own deletion ("purge temp files" right before
  purging them).
- Answers to a choice question whose probabilities add up to more than 1, beyond what rounding to
  two decimals explains (0.005 per positive entry), are rejected as invalid, for every linter and
  backend; a sum just above 1 within that rounding is normalized to 1 before rules decide.
- New linter `read-only-promise`: a doc comment that promises a function changes nothing while its
  body writes the receiver, a parameter or a package-level variable.
- `normal-event-at-error` stops reading a structured failure as a routine event: in
  `logger.Error("closing the listener", zap.Error(err))` the message only names the operation and
  the error says it failed. It now sends whether the call logs a value of type error and what the
  code checked about that error where it logs (not nil, `errors.Is(err, context.Canceled)`,
  `err == io.EOF`, `os.IsNotExist(err)`), or the error it logs by name. When the code checked the
  error is set without singling it out, a second question asks whether the message only names an
  action; if it does, the line is taken as a failure report and is not a finding, a recall
  trade-off.
- `test-name-vs-assertion` no longer reads a failure the test arranges, or an error passed in, as
  the call's own: the request now carries the test's doc comment and whether the test passes the
  call an error, and the question asks about the call itself. Version 2.
- `doc-vs-table` no longer reports a row whose case name only labels its input: a second question
  asks whether the name states the facts the doc's condition depends on, and a contradiction found
  in a name that does not abstains. Version 2.
- `suppression-rationale` (version 3) compares a reason that names a gosec, staticcheck or revive
  rule (`G304`, `SA1019`, `var-naming:`) with that rule's own title from the linter's
  documentation, asks whether the reason argues about the reported thing rather than whether it
  mentions it, and abstains below 0.9 instead of 0.8. A reason naming several rules or a rule it
  does not know is unsupported. On 25 repositories its 19 findings were all reasons that answer
  their rule, most of them by saying where a path or a command comes from; now there is one.

## v0.1.0 (2026-10-02)

First release.

- `lintuition run` over `go/packages` with `go/analysis` analyzers, a golangci-lint v2-shaped
  config decoded strictly, and text, json, sarif, checkstyle, code-climate, junit-xml and
  github-actions (extension) output.
- Linters: `destructive-remediation`, `doc-vs-table`, `enum-comment-shift`, `error-needs-type`
  (policy, off by default), `human-unit-contradiction`, `log-key-value-role`, `log-sensitive-field`,
  `metric-type-vs-help`, `normal-event-at-error`, `premature-success`, `sentinel-name-vs-text`,
  `severe-event-understated`, `suppression-rationale`, `table-case-vs-expectation`,
  `test-name-vs-assertion`.
- Classifiers: `jev` (TypeSafe System One), `openai` (OpenAI-compatible APIs with log
  probabilities, including a local Ollama), `claude-code` and `codex` (coding agents under your
  login), `fake` (scripted, local).
- Payload policy `facts` / `prose` / `source`, request preview, budgets, retries, answer cache,
  self-consistency votes.
- Explained `//nolint:<linter> // reason`.
- `lintuition custom` builds a binary with plugin linters and classifiers.
- `lintuition eval` scores linters over marked twins with a real classifier.
- Exit codes: 0 clean, 1 issues, 2 incomplete run.
