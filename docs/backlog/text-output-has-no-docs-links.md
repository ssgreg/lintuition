---
worth: later
where: internal/report/write.go:103
added: 2026-10-05
---
# text output has no docs links

`writeText` prints findings only. A footer listing one docs URL per linter that fired was considered and
not done: on by default it changes the one-line-per-finding stdout that scripts may count on (with source
lines and stats turned off), and a URL needs the network the `explain` command above does not.

Unknown that settles it: whether anyone asks for links in the text output once `explain` exists. If they
do, add it opt-in, following the text destination rather than stdout, from the final issues only.
