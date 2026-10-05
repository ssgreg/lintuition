# Changelog

## Unreleased

- A finding from a built-in linter links to that linter's section in `docs/linters.md`: `helpUri`
  on the SARIF rule, a second line in the GitHub Actions annotation, and `DocURL` on the linter in
  the JSON report's `Linters`. A release build links to the docs at its tag, any other build to
  `main`; a plugin linter has no docs here and gets no link. Text output is unchanged.

## v0.2.0 (2026-10-05)

Two new linters, and the false positives of the first ones fixed. On 25 real repositories (16
public, 9 private) the standard linters made 27 true findings and 97 false ones with v0.1.0; each
linter below was fixed against that set and a held-out set written after its design.

New linters:

- `doc-vs-signature`: a doc comment that promises a returned result or error the function's
  signature does not have. An HTTP handler whose doc says "returns X" about the response it writes
  is told apart from a value or an error promised to the Go caller.
- `read-only-promise`: a doc comment that promises a function changes nothing while its body writes
  the receiver, a parameter or a package-level variable. It stands on a new effects model,
  `internal/effects`, which finds the writes a caller can see.

Fewer false positives:

- `destructive-remediation` (version 3) no longer reads debug and info logs, where the program
  narrates its own steps ("purge temp files" right before purging them), and has an answer for a
  message that names the program's own action. Its 35 findings on the 25 repositories were all
  of that kind; now there are none.
- `suppression-rationale` (version 3) compares a reason that names a gosec, staticcheck or revive
  rule (`G304`, `SA1019`, `var-naming:`) with that rule's own title, asks whether the reason
  argues about the reported thing rather than whether it mentions it, and abstains below 0.9.
  19 findings, all reasons that answer their rule, became one.
- `severe-event-understated` (version 2) tells an absent optional file, a requested stop or
  cancel and the program's own recovery from lost work, with what the code checked on the way to
  the log (`errors.Is`, `os.IsNotExist`, a context's Done channel, a signal channel).
- `normal-event-at-error` (version 3) reads the error a log call carries: in
  `logger.Error("closing the listener", zap.Error(err))` the message names the operation and the
  error says it failed. It sends what the code checked about that error where it logs.
- `enum-comment-shift` (version 3) masks its own constant's name only where a comment opens with
  it, marks group comments and near-identical neighbours as unsupported, and ignores `// want`
  test marks.
- `test-name-vs-assertion` (version 2) no longer reads a failure the test arranges, or an error
  it passes in, as the call's own; it reads the test's doc and whether the call gets an error.
- `doc-vs-table` (version 2) abstains when a case name only labels its input.
- `table-case-vs-expectation` is unchanged: its 3 false findings depend on the input value, which
  is Go source and is not sent.

Classifiers and decisions:

- A choice answer whose probabilities add up to more than 1, beyond what rounding to two decimals
  explains, is rejected for every linter and backend; a sum just above 1 is normalized, and
  probabilities are added in a fixed order, so a cached answer decides the same way every time.
- `openai`: `reasoning-effort`, so a model that thinks first (qwen3.x, gemma4 on Ollama) answers
  with its first token; a confident answer no longer fails on a confidence a rounding error
  above 1.

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
