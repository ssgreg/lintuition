---
worth: later
where: linters/docsignature
added: 2026-10-05
---
# doc-vs-signature excuses a stale return promise whenever a response writer is around

A handler doc says it "returns the user", the function returns nothing, and the finding is dropped
because an `http.ResponseWriter` parameter could be how it returns. The writer may be unrelated. A
general `io.Writer` excuse was rejected earlier: `testing.T`, `analysis.Pass` and log writers hid real
stale docs. On the 25-repo precision run 7 handler docs ended as unclear.

Unknown that settles it: a narrow fact that the function writes a response through that same writer,
delegation included, without losing the correct handler negatives.
