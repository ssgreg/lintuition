---
worth: later
where: classifiers
added: 2026-10-05
---
# no local embedding-based classifier backend

For fixed Choice, yes-or-no and Score questions, an embedding model fine-tuned for decisions may answer
in the right format where a small local generative model does not. It would start as an external plugin,
not a model runtime in the Go release.

Unknown that settles it: latency, memory, recall, false positives and abstention on the frozen twin and
precision sets, plus negations, paraphrases and the input length limit. Its probabilities are not
comparable to another backend's without measuring.
