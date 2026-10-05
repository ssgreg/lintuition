---
worth: no
added: 2026-10-05
---
# no linter for a comment that claims a visible effect the code does not have

`// The row is no longer rendered.` over `delete(visible, id)`.

The prototype (`comment-ui-claim`) asked whether a comment claims something is shown or hidden while the
code only changes a set. Its one finding on an internal service was false: the comment described a
screen the program reads, literally.

Rejected. The set can be the renderer's input, and the absence of rendering code near the comment is no
evidence the claim is false. The question also needed the code under the comment, which is source.