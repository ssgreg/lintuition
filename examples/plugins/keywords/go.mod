// An example third-party classifier for lintuition: deterministic and local.
module example.com/lintuition-keywords

go 1.26.5

require github.com/ssgreg/lintuition v0.0.0

require golang.org/x/tools v0.50.0 // indirect

replace github.com/ssgreg/lintuition => ../../..
