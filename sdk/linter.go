package sdk

import (
	"fmt"
	"go/token"
	"reflect"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

// A linter has two halves. Its Analyzer runs locally over typed syntax and returns candidates; its
// Rule asks the classifier narrow questions about a candidate and decides in Go code. The classifier
// judges meaning, the rule judges logic, and a backend never decides whether something is a finding.

// CandidatesType is the ResultType every linter Analyzer must declare.
var CandidatesType = reflect.TypeFor[[]*Candidate]()

// Candidate is one place a linter wants to ask about.
type Candidate struct {
	Pos token.Position
	// Subject names what the candidate is about (a metric, a function); it is local and never sent.
	Subject string
	// Payload is what may be sent to the classifier, sorted by how private each part is.
	Payload Payload
	// Local holds facts the rule's Decide needs that are never sent.
	Local map[string]string
	// Unsupported, when set, says why the analyzer saw the shape but could not extract the facts (a
	// message built at run time, say). Such a candidate is never asked; it is counted as unsupported, so
	// a run shows what it did not cover instead of looking clean.
	Unsupported string
}

// Payload is the outbound state of a candidate. Every field is one of three kinds, and the configured
// payload policy decides which kinds may leave the machine.
type Payload struct {
	// Facts are structural facts computed by code: types, kinds, counts, booleans, enum names.
	Facts map[string]any
	// Prose is text a person wrote: comments, metric Help, log messages, test case names.
	Prose map[string]string
	// Source is Go source text. The default policy never sends it.
	Source map[string]string
}

// Fact adds a structural fact.
func (p *Payload) Fact(key string, v any) {
	if p.Facts == nil {
		p.Facts = map[string]any{}
	}
	p.Facts[key] = v
}

// AddProse adds person-written text.
func (p *Payload) AddProse(key, text string) {
	if p.Prose == nil {
		p.Prose = map[string]string{}
	}
	p.Prose[key] = text
}

// AddSource adds Go source text.
func (p *Payload) AddSource(key, text string) {
	if p.Source == nil {
		p.Source = map[string]string{}
	}
	p.Source[key] = text
}

// Decision is a rule's verdict on one candidate.
type Decision struct {
	// Report is true when the candidate is a finding.
	Report  bool
	Message string
	// Abstained is the reason the rule could not decide; empty when it decided.
	Abstained string
}

// Report returns a finding with a message.
func Report(format string, args ...any) Decision {
	return Decision{Report: true, Message: fmt.Sprintf(format, args...)}
}

// Clean returns a decision that the candidate is fine.
func Clean() Decision { return Decision{} }

// Abstain returns a decision that the rule cannot tell, with the reason.
func Abstain(reason string) Decision { return Decision{Abstained: reason} }

// Rule is the semantic half of a linter.
type Rule interface {
	// Questions returns what to ask about a candidate; none means the candidate is skipped.
	Questions(c *Candidate) []Question
	// Decide turns the answers, keyed by question ID, into a decision.
	Decide(c *Candidate, answers map[string]Answer) Decision
}

// Linter is a registered linter.
type Linter struct {
	Name string
	Doc  string
	// Standard puts the linter in the `standard` default set.
	Standard bool
	// Analyzer extracts candidates; its ResultType must be CandidatesType.
	Analyzer *analysis.Analyzer
	// NewSettings returns a pointer to a zero settings struct; nil means the linter takes no settings.
	NewSettings func() any
	// New builds the rule from decoded settings (nil when NewSettings is nil).
	New func(settings any) (Rule, error)
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Validate reports a malformed linter definition.
func (l Linter) Validate() error {
	if !nameRE.MatchString(l.Name) {
		return fmt.Errorf("linter name %q must be lower-case letters, digits and dashes", l.Name)
	}
	if l.Analyzer == nil || l.New == nil {
		return fmt.Errorf("linter %s: Analyzer and New are required", l.Name)
	}
	if l.Analyzer.ResultType != CandidatesType {
		return fmt.Errorf("linter %s: Analyzer.ResultType must be sdk.CandidatesType", l.Name)
	}
	return nil
}
