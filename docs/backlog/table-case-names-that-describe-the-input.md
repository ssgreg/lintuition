---
worth: later
where: linters/tablecase/tablecase.go:145
added: 2026-10-05
---
# table case names that describe the input read as the expected result

Three false positives on the 25-repo precision run, all one shape: the case name describes the input,
not the outcome. In alertmanager's `api/v2/api_test.go`, cases "not equal match" and "not regex match"
name the matcher operator and expect `true`; the linter reads "not ... match" as an expected `false`. A
third case was named after its input state (a deleted record with a flag on) and expected `false`.

No fix found yet. Unknown that settles it: a question or a code-side rule that separates "names the input" from
"names the result" on the held-out set without losing recall.
