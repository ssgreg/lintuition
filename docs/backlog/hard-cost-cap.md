---
worth: later
where: docs/classifiers.md:29
added: 2026-10-05
---
# the cost limit is not a hard cap on the bill

`semantic.budget.max-cost-usd` stops new questions once reported spending reaches the limit; requests in
flight finish, so the total can end above it, as `docs/classifiers.md` says. A hard cap would reserve a
known upper cost before each request, retries and requests in flight included.

Unknown that settles it: an upper price per request, which many backends do not give. Where it is
missing, a hard-cap mode must be unavailable, not approximate.
