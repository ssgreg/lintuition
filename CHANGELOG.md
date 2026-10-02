# Changelog

## Unreleased

- New linter `doc-vs-signature`: a doc comment that promises a returned result or error the
  function's signature does not have.
- `destructive-remediation` (version 3) no longer reads debug and info logs, sends the log level
  and the function called right after a log call, has an answer for a message that names the
  program's own action, and counts its three non-destructive answers together. On 25 repositories
  its 35 findings were all a program announcing its own deletion ("purge temp files" right before
  purging them).
- Answers to a choice question whose probabilities add up to more than 1 (beyond 0.005 per option
  of rounding) are rejected as invalid, for every linter and backend.

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
