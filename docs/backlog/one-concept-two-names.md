---
worth: later
rank: 630
added: 2026-10-05
---
# no linter for one concept under two names, or one name for two concepts

A package that says `tenant` in some places and `customer` in others for the same thing, or a function
that calls one object "snapshot" in one log line and "volume" in the next.

Never run. It covers terminology across a package, noun drift inside one function, and one term used
with two meanings. Go code would find candidate pairs; the classifier would judge "same concept?". A
scan of two internal services found no glossary, so no candidate had a reference meaning to compare
with.

Unknown that settles it: where the reference meaning comes from (a glossary file the team keeps?) and
whether pairs found by code are precise enough to ask about.
