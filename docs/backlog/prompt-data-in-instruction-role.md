---
worth: later
rank: 660
added: 2026-10-05
---
# no linter for user data passed as a model instruction

User input passed as the system or instruction field of an LLM SDK call, instead of as user content.

It needs a typed SDK constructor and a proven data path; without them the candidate is unsupported. It
would be a security plugin, and it would not guarantee protection from prompt injection. Never run.

Unknown that settles it: which SDKs are common enough in Go code to model, and whether the classifier
adds anything once the path is proven.
