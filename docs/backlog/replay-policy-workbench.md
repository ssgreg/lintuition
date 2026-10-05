---
worth: yes
where: internal/classify
added: 2026-10-05
---
# no way to try a threshold or policy on saved answers

Changing a threshold, a margin or an abstain rule today means paying for a new run. Saved answers
already carry full probability distributions, so a replay command could recompute decisions on them and
show, per case, what moved between finding, clean and abstained.

The work: a reproducible command that first reproduces the current `Decide` exactly, then sweeps, on the
tuning split only. A new question text, option set or payload still needs new answers; this does not
make an incompatible cache reusable.
