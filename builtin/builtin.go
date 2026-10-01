// Package builtin registers lintuition's built-in linters and classifiers. A custom binary imports it
// next to its plugins.
package builtin

import (
	_ "github.com/ssgreg/lintuition/classifiers/fake"
	_ "github.com/ssgreg/lintuition/linters/metrictypevshelp"
)
