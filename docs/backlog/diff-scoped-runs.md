---
worth: yes
added: 2026-10-05
---
# no way to pay only for candidates a change touches

On a pull request, most candidates did not change, and their answers are cached or irrelevant.
Diff-scoped runs would still type-check whole packages, so facts stay valid, and then ask only about
candidates the change touched. A doc changed in a neighbouring file or a shared helper can make an old
answer stale, so the report states its scope, and unchecked candidates never count as clean.

The work: whole-package extraction plus a candidate filter with a reported scope first, dependency
invalidation later. Diff mode for the comment-drift item builds on this.
