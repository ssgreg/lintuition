---
worth: yes
added: 2026-10-05
---
# no way to turn a team's written rule into a check without writing Go

A rules file says "a test name must describe the scenario, not only the function", and lintuition asks
that question of every test name. Today a team writes a plugin linter in Go, see `docs/plugins.md`.

The shape that fits lintuition is two mechanisms side by side. Go keeps what only code can do: finding
candidates and proving facts about them (`internal/effects`, error slots, unit conversions). A rule in
`.lintuition.yml` names a candidate kind, a question with its options, the option that reports, a
threshold, and an optional condition over the candidate's facts:

```yaml
semantic:
  rules:
    - id: doc-promises-an-error
      on: func-doc
      question: "Does the doc say the function returns an error?"
      options: {yes: "...", no: "..."}
      report: yes
      when: "!facts.has_error_result"
      message: "doc says {function} returns an error, but it has no error result"
```

No source leaves the machine: a rule sees only its candidate's text and the facts that kind exposes.
Linters with complex decisions stay in Go.

A first try (five policy questions, 25 regex-extracted candidates per kind on two repos, 243 requests)
showed Jev answering the rule as worded, and that the facts decide usefulness: 5 of 6 "generic error
message" hits were in test files, which a `test file` fact and a condition remove.

Work: a registry of named candidate kinds with fixed facts (start with `func-doc`, `error-text`,
`test-name`), the rule format and a condition language, and a check that a shipped linter rewritten as a
rule matches its Go version on the twins.
