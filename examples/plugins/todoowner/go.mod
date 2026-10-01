// An example third-party linter for lintuition. It lives in its own module and imports only the
// public sdk package.
module example.com/lintuition-todo-owner

go 1.26.5

require (
	github.com/ssgreg/lintuition v0.0.0
	golang.org/x/tools v0.50.0
)

replace github.com/ssgreg/lintuition => ../../..
