---
worth: later
where: internal/classify/cache.go:39
added: 2026-10-05
---
# the answer cache has no size limit

The cache has a TTL and an explicit clean, which limit how old an answer may be but not how much disk
the cache takes. A size bound would evict only lintuition's own record files, keep writes atomic, and
never delete foreign files when `Dir` points somewhere wrong.

Unknown that settles it: how fast the cache grows in real CI with cache reuse. Pick a predictable policy
before reaching for an LRU nobody needs.
