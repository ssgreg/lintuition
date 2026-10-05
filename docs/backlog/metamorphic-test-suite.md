---
worth: yes
where: linttest
added: 2026-10-05
---
# no shared suite of transformations that must not change a decision

The same regressions kept coming back in new extractors: reordering fields or choice keys changed a
decision, harmless formatting changed fallthrough, `go`, `defer` or an immediately called function
literal took a different walk, a long name broke the fact screen, a `want` comment hinted the right
label to the model.

The work: collect those transformations into one reusable check that compares bindings and state, not
only findings. Score the layers apart too (fact extraction, the meaning of the prose, the final policy),
so a better classifier score cannot hide a wrong binding.
