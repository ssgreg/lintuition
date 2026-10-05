---
worth: later
rank: 190
where: linters/testnameassert
added: 2026-10-05
---
# no finding when a test's name and its doc say opposite things

`// Rejects a token without a signature.` above `func TestValidateAcceptsUnsignedToken`.

`test-name-vs-assertion` reads the doc when the name only describes the input. That helped setup-only
names, but a doc can also drown a correct name: a kept stale-doc defect was missed in 0 of 3 runs. The
idea is to classify the name and the doc independently and report their disagreement as a hint, without
deciding which one is wrong.

Unknown that settles it: whether both directions can be judged independently without the name leaking
into the doc's payload as a hint, and how often real tests carry both.
