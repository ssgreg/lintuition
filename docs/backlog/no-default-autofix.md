---
worth: no
added: 2026-10-05
---
# no default autofix from a semantic finding

A finding says two things disagree, not which one is wrong, so lintuition does not rename, rewrite a
doc, change a log level or add a `nolint` by default. A wording finding such as a "returns" doc on a
method that sends on a channel is sometimes a policy question, not a bug. Suggesting checkable options
is fine. What would reopen this: a narrow class with an unambiguous fix and an independent check of the
result, not a high score.
