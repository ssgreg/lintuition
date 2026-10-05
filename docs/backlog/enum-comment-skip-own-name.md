---
worth: no
where: linters/enumcomment
added: 2026-10-05
---
# do not skip enum comments that name their own constant

Skipping every comment that starts with its own constant's name looks like an easy fix for
`enum-comment-shift` false positives. Rejected: it removes the ordinary Go doc form and the stale
description under a correct name. On x/tools it cut candidates from 325 to 122, and on an internal
service from 26 to 1. The narrower fix is in: only the opening name is masked. Two comments that differ
only in case or punctuation are still not one piece of evidence.
