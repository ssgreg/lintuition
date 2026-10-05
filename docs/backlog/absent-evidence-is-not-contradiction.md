---
worth: yes
where: docs/plugins.md
added: 2026-10-05
---
# no written rule that missing local evidence is not a contradiction

"No `Lock` call here, so the lock comment is wrong", "no renderer here, so the hide comment is wrong",
"no local cause, so the error's cause is made up": each of these reads absence as contradiction. A
caller precondition, an implementation elsewhere or an abstraction makes all three legal. The same goes
for a non-error interface result, which can hold an error value.

The work: write the rule into the plugin authoring docs, and label eval cases by kind (direct
contradiction, explicit over-promise, allowed incompleteness, unclear), with Go policy deciding which
kinds may become findings. A finding needs a positive counterexample; without one, the candidate is
unsupported or abstains.
