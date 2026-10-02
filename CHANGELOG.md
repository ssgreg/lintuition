# Changelog

## Unreleased

- `openai`: `reasoning-effort`, so a model that thinks first (qwen3.x, gemma4 on Ollama) answers
  with its first token; a confident answer no longer fails on a confidence a rounding error above 1.
- New linter `doc-vs-signature`: a doc comment that promises a returned result or error the
  function's signature does not have.

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
