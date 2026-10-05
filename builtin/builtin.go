// Package builtin registers lintuition's built-in linters and classifiers. A custom binary imports it
// next to its plugins.
package builtin

import (
	_ "github.com/ssgreg/lintuition/classifiers/agentcli"
	_ "github.com/ssgreg/lintuition/classifiers/fake"
	_ "github.com/ssgreg/lintuition/classifiers/jev"
	_ "github.com/ssgreg/lintuition/classifiers/openai"
	"github.com/ssgreg/lintuition/internal/report"
	"github.com/ssgreg/lintuition/linters/destructiveadvice"
	"github.com/ssgreg/lintuition/linters/docsignature"
	"github.com/ssgreg/lintuition/linters/docvstable"
	"github.com/ssgreg/lintuition/linters/enumcomment"
	"github.com/ssgreg/lintuition/linters/errorneedstype"
	"github.com/ssgreg/lintuition/linters/humanunit"
	"github.com/ssgreg/lintuition/linters/logkeyrole"
	"github.com/ssgreg/lintuition/linters/logsensitive"
	"github.com/ssgreg/lintuition/linters/metrictypevshelp"
	"github.com/ssgreg/lintuition/linters/normalaterror"
	"github.com/ssgreg/lintuition/linters/prematuresuccess"
	"github.com/ssgreg/lintuition/linters/readonlypromise"
	"github.com/ssgreg/lintuition/linters/sentinelname"
	"github.com/ssgreg/lintuition/linters/severeunderstated"
	"github.com/ssgreg/lintuition/linters/suppressionreason"
	"github.com/ssgreg/lintuition/linters/tablecase"
	"github.com/ssgreg/lintuition/linters/testnameassert"
)

// Marks the linters registered above as built-in, so their findings link to their docs. A linter
// imported here and left out of the list registers without a link; a test catches that.
func init() {
	for _, name := range []string{
		destructiveadvice.Name,
		docsignature.Name,
		docvstable.Name,
		enumcomment.Name,
		errorneedstype.Name,
		humanunit.Name,
		logkeyrole.Name,
		logsensitive.Name,
		metrictypevshelp.Name,
		normalaterror.Name,
		prematuresuccess.Name,
		readonlypromise.Name,
		sentinelname.Name,
		severeunderstated.Name,
		suppressionreason.Name,
		tablecase.Name,
		testnameassert.Name,
	} {
		report.MarkBuiltin(name)
	}
}
