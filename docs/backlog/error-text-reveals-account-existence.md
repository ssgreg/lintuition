---
worth: later
rank: 300
added: 2026-10-05
---
# no linter for an error that tells an attacker which part of a login was wrong

`http.Error(w, "That email is registered, but its password is wrong", http.StatusUnauthorized)`.

The prototype (`security-overdisclosure`) asked whether the text reveals that an account exists. No
recorded run produced a finding. Building the error says nothing about where it goes. A private log line
may say this; a public unauthenticated response must not.

Unknown that settles it: sink facts (this text reaches an unauthenticated response) and a policy that
says when uniform responses are required.
