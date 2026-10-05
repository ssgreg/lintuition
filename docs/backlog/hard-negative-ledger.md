---
worth: yes
where: testdata/twins
added: 2026-10-05
---
# no ledger of how hard cases and their labels changed

A hard negative rewritten until the model passes disappears from the metric along with the error it
caught. The ledger keeps the original case, the fixed control, the reason the label changed and who
decided it, and the report covers every original label.

It also keeps apart what is easy to mix up: abstained and clean, repeated runs and independent cases, an
incomplete corpus and a full one. Real defects taken from public issue and commit history can join it as
independent cases; a case ported from another language is marked synthetic, and anything used for tuning
stays out of the held-out set.

The work: a machine-readable journal of case origin and status next to `testdata/twins`.
