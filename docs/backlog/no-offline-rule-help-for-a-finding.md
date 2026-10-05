---
worth: yes
where: cli/cli.go:210
added: 2026-10-05
---
# no offline way to read a linter's rule from a finding

A finding links to its section in `docs/linters.md` only in SARIF, GitHub Actions and JSON. The text
output, which is what a local coding agent reads, has no pointer to the rule. A URL would not fully help
there anyway: it needs the network, and for a dev build it points at `main`, which may have moved on.

The plan: `lintuition explain <linter>` prints that linter's section from `docs/linters.md` embedded in
the binary. It reads no config, loads no packages and makes no requests. A same-name plugin gets no
built-in section. After the summary, and only when text findings were shown, one line on stderr names
the command. stdout keeps its format.

Effort: a new command and a small embed package next to `docs/linters.md`, plus a test that every
built-in's section is found whole.
