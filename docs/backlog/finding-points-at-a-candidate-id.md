---
worth: later
where: sdk/linter.go:20
added: 2026-10-05
---
# a group finding cannot point at the member that conflicts

A future package or type linter may find a conflict among several members and report it at the first
line of the file. The extractor could hand the classifier a fixed list of candidate IDs with local
positions; the classifier picks an ID or unknown, and Go code checks the ID belongs to the group. No
file and no arbitrary line number is sent.

Unknown that settles it: a group linter that needs it. Confidence in a conflict is also not confidence
in its place, so the two need separate thresholds.
