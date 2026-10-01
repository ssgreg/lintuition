// Package builtin registers lintuition's built-in linters and classifiers. A custom binary imports it
// next to its plugins.
package builtin

import (
	_ "github.com/ssgreg/lintuition/classifiers/agentcli"
	_ "github.com/ssgreg/lintuition/classifiers/fake"
	_ "github.com/ssgreg/lintuition/classifiers/jev"
	_ "github.com/ssgreg/lintuition/classifiers/openai"
	_ "github.com/ssgreg/lintuition/linters/destructiveadvice"
	_ "github.com/ssgreg/lintuition/linters/logkeyrole"
	_ "github.com/ssgreg/lintuition/linters/logsensitive"
	_ "github.com/ssgreg/lintuition/linters/metrictypevshelp"
	_ "github.com/ssgreg/lintuition/linters/normalaterror"
	_ "github.com/ssgreg/lintuition/linters/prematuresuccess"
	_ "github.com/ssgreg/lintuition/linters/severeunderstated"
	_ "github.com/ssgreg/lintuition/linters/tablecase"
)
