// Package engine runs a lint: load, extract, ask, decide, report.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/internal/classify"
	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/load"
	"github.com/ssgreg/lintuition/internal/report"
	"github.com/ssgreg/lintuition/sdk"
)

// Options are a run's inputs besides the config.
type Options struct {
	Config   *config.Config
	Dir      string
	Patterns []string
	// DryRun extracts and plans requests, but sends none.
	DryRun bool
	// Preview, when set in a dry run, gets one JSON line per planned request: where it comes from and
	// what would be sent. It stays on this machine; it may hold private prose.
	Preview io.Writer
	Log     io.Writer
}

// Result is a run's outcome.
type Result struct {
	Issues []report.Issue
	Run    report.Run
	// Files are the analysed Go files, absolute.
	Files []string
}

// Run executes one lint.
func Run(ctx context.Context, o Options) (*Result, error) {
	c := o.Config
	enabled, err := c.Select(sdk.Linters())
	if err != nil {
		return nil, err
	}
	cl, clName, err := c.Classifier(sdk.Classifiers())
	if err != nil {
		return nil, err
	}
	if cl == nil && len(enabled) > 0 && !o.DryRun {
		return nil, errors.New("semantic.classifier is not set; every enabled linter asks a classifier (see `lintuition classifiers`)")
	}
	if cl != nil && !cl.Capabilities().Local {
		caps := cl.Capabilities()
		switch {
		case c.Semantic.Budget.MaxCostUSD > 0 && !caps.CostKnown:
			// A cap the run cannot see the spending against would pass for protection.
			return nil, fmt.Errorf("semantic.budget.max-cost-usd is set, but classifier %s does not know what its calls cost: set its price-per-mtok", clName)
		case c.Semantic.Budget.MaxCostUSD == 0 && caps.CostKnown && o.Log != nil:
			fmt.Fprintf(o.Log, "lintuition: classifier %s is paid and semantic.budget.max-cost-usd is not set: spending is not capped\n", clName)
		case c.Semantic.Budget.MaxCostUSD == 0 && o.Log != nil:
			fmt.Fprintf(o.Log, "lintuition: classifier %s sends requests off the machine and its cost is unknown: set price-per-mtok and semantic.budget.max-cost-usd to cap it\n", clName)
		}
	}
	if rd, ok := cl.(interface{ Ready() error }); ok && !o.DryRun {
		if err := rd.Ready(); err != nil {
			return nil, fmt.Errorf("classifier %s: %w", clName, err)
		}
	}
	proc, err := report.NewProcessor(c)
	if err != nil {
		return nil, err
	}
	if o.Log == nil {
		o.Log = io.Discard
	}
	if c.Run.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeDuration(c.Run.Timeout))
		defer cancel()
	}

	analyzers := make([]*analysis.Analyzer, 0, len(enabled))
	for _, e := range enabled {
		analyzers = append(analyzers, e.Linter.Analyzer)
	}
	loaded, err := load.Run(ctx, load.Options{
		Dir: o.Dir, Patterns: o.Patterns, Tests: c.Tests(), BuildTags: c.Run.BuildTags, ModFlag: c.Run.ModulesDownloadMode,
	}, analyzers)
	if err != nil {
		return nil, err
	}
	base, err := relBase(c, o.Dir)
	if err != nil {
		return nil, err
	}

	var cache *classify.Cache
	if id, ok := cl.(classify.Identifier); ok && !c.Semantic.Cache.Disabled && id.Identity() != "" {
		dir := c.Semantic.Cache.Dir
		if dir == "" {
			if dir, err = classify.DefaultCacheDir(); err != nil {
				return nil, err
			}
		}
		cache = &classify.Cache{Dir: dir, TTL: timeDuration(c.Semantic.Cache.TTL)}
	}
	r := &runner{cache: cache, votes: c.Semantic.Votes,
		cl: cl, clName: clName, policy: c.Semantic.Payload, budget: c.Semantic.Budget,
		sem: make(chan struct{}, c.Semantic.Concurrency), dryRun: o.DryRun, previewW: o.Preview, log: o.Log,
	}
	res := &Result{}
	res.Run.Stats.Packages = loaded.Packages
	res.Files = loaded.Files
	for _, p := range loaded.Problems {
		res.Run.Problems = append(res.Run.Problems, fmt.Sprintf("package %s not analysed: %s", p.Package, p.Err))
	}
	generated := map[string]bool{}
	notAsked := 0
	nolint := report.NewNolint()
	for _, e := range enabled {
		st := report.LinterStatus{Name: e.Linter.Name, Enabled: true}
		var jobs []*job
		for _, cand := range loaded.Candidates[e.Linter.Analyzer] {
			// Files are read by their absolute name; only the reported name is made relative.
			abs := cand.Pos.Filename
			cand.Pos.Filename = rel(base, abs)
			st.Candidates++
			if cand.Unsupported != "" {
				st.Unsupported++
				continue
			}
			if proc.Covers(e.Linter.Name, cand.Pos.Filename) {
				st.Skip("excluded")
				continue
			}
			if isGenerated(c, generated, abs) {
				st.Skip("generated")
				continue
			}
			// An explained //nolint for this linter on this line: no need to ask, nothing is sent.
			if nolint.Covers(abs, e.Linter.Name, cand.Pos.Line) {
				st.Skip("nolint")
				continue
			}
			jobs = append(jobs, &job{linter: e.Linter, rule: e.Rule, cand: cand})
		}
		r.do(ctx, jobs)
		// Identical findings (same subject and text in one file) are told apart by their order in
		// the file, which unrelated edits elsewhere do not change.
		seenFP := map[string]int{}
		for _, j := range jobs {
			fpKey := j.cand.Pos.Filename + "\x00" + j.cand.Subject + "\x00" + j.decision.Message
			occurrence := seenFP[fpKey]
			seenFP[fpKey]++
			switch {
			case j.notAsked:
				st.NotAsked++
				notAsked++
			case j.planned:
				st.Planned++
			case j.skipped != "":
				st.Skip(j.skipped)
			case j.err != nil:
				st.Failed++
				res.Run.Problems = append(res.Run.Problems, fmt.Sprintf("%s %s:%d: %v", e.Linter.Name, j.cand.Pos.Filename, j.cand.Pos.Line, j.err))
			case j.asked && j.decision.Abstained != "":
				st.Asked++
				st.Abstained++
				res.Run.Abstentions = append(res.Run.Abstentions, report.Abstention{
					Linter: e.Linter.Name, File: j.cand.Pos.Filename, Line: j.cand.Pos.Line, Reason: j.decision.Abstained,
				})
			case j.asked:
				st.Asked++
				if j.decision.Report {
					res.Issues = append(res.Issues, report.Issue{
						FromLinter: e.Linter.Name, Text: j.decision.Message, Pos: j.cand.Pos,
						Evidence:    evidence(clName, r.model(), e.Linter.Version, j),
						Fingerprint: fingerprint(e.Linter.Name, j.cand.Pos.Filename, j.cand.Subject, j.decision.Message, occurrence),
					})
				}
			}
		}
		res.Run.Linters = append(res.Run.Linters, st)
	}
	for _, l := range sdk.Linters() {
		if !hasStatus(res.Run.Linters, l.Name) {
			res.Run.Linters = append(res.Run.Linters, report.LinterStatus{Name: l.Name})
		}
	}
	sort.Slice(res.Run.Linters, func(i, j int) bool { return res.Run.Linters[i].Name < res.Run.Linters[j].Name })
	if notAsked > 0 {
		hint := ""
		if cache != nil {
			hint = "; the answers already received are cached, so a run with a higher cap pays only for the rest"
		}
		res.Run.Problems = append(res.Run.Problems, fmt.Sprintf("%d candidate(s) not asked: %s; the findings above are from the candidates that were%s", notAsked, r.cappedReason(), hint))
	}
	res.Run.Problems = append(res.Run.Problems, r.problems()...)
	res.Run.Incomplete = len(res.Run.Problems) > 0 || ctx.Err() != nil
	if ctx.Err() != nil {
		res.Run.Problems = append(res.Run.Problems, "run stopped: "+ctx.Err().Error())
	}
	res.Run.Stats.Requests, res.Run.Stats.InputTokens, res.Run.Stats.CostUSD = r.sent, r.tokens, r.cost
	if o.DryRun {
		res.Run.Stats.Requests = r.requests // planned
	}
	res.Run.Stats.CacheHits, res.Run.Stats.Votes = r.hits, r.votes
	proc.Base = base
	res.Issues = proc.Process(res.Issues)
	return res, nil
}

type job struct {
	linter sdk.Linter
	rule   sdk.Rule
	cand   *sdk.Candidate

	asked     bool
	planned   bool
	notAsked  bool // the budget ran out before this candidate
	replayed  bool
	samples   int
	agreement map[string]string
	perSample map[string][]map[string]float64
	key       string
	skipped   string
	err       error
	answers   map[string]sdk.Answer
	decision  sdk.Decision
}

type runner struct {
	cl       sdk.Classifier
	clName   string
	cache    *classify.Cache
	votes    int
	policy   string
	budget   config.Budget
	sem      chan struct{}
	dryRun   bool
	previewW io.Writer
	log      io.Writer

	mu          sync.Mutex
	requests    int // reserved
	sent        int // attempted
	hits        int
	tokens      int
	texts       map[string]string
	unknownCost int
	cost        float64
	capped      string
}

func (r *runner) do(ctx context.Context, jobs []*job) {
	var wg sync.WaitGroup
	for _, j := range jobs {
		qs := j.rule.Questions(j.cand)
		if len(qs) == 0 {
			j.skipped = "no-questions"
			continue
		}
		if err := r.plan(qs); err != nil {
			j.err = err
			continue
		}
		if err := r.stable(j.linter.Name, qs); err != nil {
			j.err = err
			continue
		}
		state, err := classify.State(j.cand.Payload, r.policy)
		var perr classify.ErrPolicy
		if errors.As(err, &perr) {
			j.skipped = "payload-policy"
			continue
		}
		if err != nil {
			j.err = err
			continue
		}
		req := sdk.Request{Linter: j.linter.Name, State: state, Questions: qs}
		if r.cache != nil {
			key, err := classify.Key(r.cl.(classify.Identifier).Identity(), j.linter.Name+"@"+j.linter.Version, r.votes, req)
			if err != nil {
				j.err = err
				continue
			}
			j.key = key
			if bd, ok := r.cache.Lookup(key, r.votes, qs); ok {
				// A replay: the same samples as before, not new votes.
				r.mu.Lock()
				r.hits++
				r.mu.Unlock()
				j.replayed = true
				if r.dryRun {
					continue
				}
				r.decide(j, req, bd.Samples)
				continue
			}
		}
		if !r.reserve(r.votes * r.callsPer(qs)) {
			j.notAsked = true
			continue
		}
		if r.dryRun {
			if err := r.preview(j, req); err != nil {
				j.err = err
				continue
			}
			j.planned = true
			continue
		}
		wg.Add(1)
		r.sem <- struct{}{}
		go func() {
			defer func() { <-r.sem; wg.Done() }()
			r.ask(ctx, j, req)
		}()
	}
	wg.Wait()
}

// bodyer is a classifier that can show the exact body it would send.
type bodyer interface {
	Body(sdk.Request) ([]byte, error)
}

// preview writes one planned request; the first failure is kept and fails the run, so a missing or
// partial preview never passes for a complete one.
func (r *runner) preview(j *job, req sdk.Request) error {
	if r.previewW == nil {
		return nil
	}
	line := map[string]any{
		"linter": j.linter.Name, "file": j.cand.Pos.Filename, "line": j.cand.Pos.Line,
		"state": req.State, "questions": req.Questions,
	}
	if b, ok := r.cl.(bodyer); ok {
		body, err := b.Body(req)
		if err != nil {
			return fmt.Errorf("preview: request body: %w", err)
		}
		line["body"] = json.RawMessage(body)
	}
	enc, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("preview: %w", err)
	}
	if _, err := r.previewW.Write(append(enc, '\n')); err != nil {
		return fmt.Errorf("preview: %w", err)
	}
	return nil
}

func (r *runner) plan(qs []sdk.Question) error {
	seen := map[string]bool{}
	for _, q := range qs {
		if err := q.Validate(); err != nil {
			return err
		}
		if seen[q.ID] {
			return fmt.Errorf("question %q asked twice in one request", q.ID)
		}
		seen[q.ID] = true
		if r.cl != nil && !r.cl.Capabilities().Supports(q.Kind) {
			return fmt.Errorf("classifier %s does not answer %s questions", r.clName, q.Kind)
		}
	}
	return nil
}

// callsPer is how many calls one sample of a request takes.
func (r *runner) callsPer(qs []sdk.Question) int {
	if r.cl != nil && r.cl.Capabilities().CallsPerQuestion {
		return len(qs)
	}
	return 1
}

// spent reports, before a sample is sent, that the observed cost leaves nothing of
// semantic.budget.max-cost-usd: at or past the cap, no further sample goes out.
func (r *runner) spent() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.budget.MaxCostUSD > 0 && (r.cost >= r.budget.MaxCostUSD || r.unknownCost > 0) {
		// An unknown cost counts as spent: the run cannot tell what is left.
		if r.capped == "" {
			r.capped = fmt.Sprintf("semantic.budget.max-cost-usd %.4f spent", r.budget.MaxCostUSD)
		}
		return true
	}
	return false
}

// used accounts one call's usage. Usage that is unknown or nonsense (negative, not finite) cannot be
// counted against a money cap: under one, the run stops spending, since it cannot tell what is left.
func (r *runner) used(u sdk.Usage) {
	bad := u.Unknown || u.InputTokens < 0 || math.IsNaN(u.CostUSD) || math.IsInf(u.CostUSD, 0) || u.CostUSD < 0
	r.mu.Lock()
	if bad {
		r.unknownCost++
		if r.budget.MaxCostUSD > 0 && r.capped == "" {
			r.capped = "a call's cost is unknown, so semantic.budget.max-cost-usd cannot be kept"
		}
		r.mu.Unlock()
		return
	}
	r.tokens += u.InputTokens
	r.cost += u.CostUSD
	r.mu.Unlock()
	r.overCost()
}

// overCost reports, and records as the reason the run is incomplete, that the observed cost has
// passed semantic.budget.max-cost-usd. A response that lands exactly on the cap is within it.
func (r *runner) overCost() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.budget.MaxCostUSD > 0 && r.cost > r.budget.MaxCostUSD {
		if r.capped == "" {
			r.capped = fmt.Sprintf("semantic.budget.max-cost-usd %.4f passed (spent %.4f)", r.budget.MaxCostUSD, r.cost)
		}
		return true
	}
	return false
}

// reserve counts n requests against the budget before they are sent; false means the budget does
// not cover all of them, and none is sent: a vote with missing samples is not a vote.
func (r *runner) reserve(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.capped != "" {
		return false
	}
	if r.budget.MaxRequests > 0 && r.requests+n > r.budget.MaxRequests {
		r.capped = fmt.Sprintf("semantic.budget.max-requests %d reached", r.budget.MaxRequests)
		return false
	}
	if r.budget.MaxCostUSD > 0 && r.cost >= r.budget.MaxCostUSD {
		r.capped = fmt.Sprintf("semantic.budget.max-cost-usd %.4f reached", r.budget.MaxCostUSD)
		return false
	}
	r.requests += n
	return true
}

// ask sends the request once per vote. Any failed or invalid sample fails the candidate: an
// operational failure is not a vote against.
func (r *runner) ask(ctx context.Context, j *job, req sdk.Request) {
	var bd classify.Bundle
	count := func() {
		r.mu.Lock()
		r.sent++
		r.mu.Unlock()
	}
	calls := 0 // calls started for the current sample
	req.Start = func() error {
		// Right before a call goes out, after the backend's own queues: the cost may be spent by now.
		if r.spent() {
			return fmt.Errorf("%w: %s", sdk.ErrBudget, r.cappedReason())
		}
		calls++
		count()
		return nil
	}
	req.Retry = func() error {
		if !r.reserve(1) {
			return fmt.Errorf("%w: %s", sdk.ErrBudget, r.cappedReason())
		}
		return nil
	}
	reported := false
	req.Used = func(u sdk.Usage) {
		reported = true
		r.used(u)
		bd.Usage.InputTokens += u.InputTokens
		bd.Usage.CostUSD += u.CostUSD
	}
	for i := range r.votes {
		// The cost of the samples already back may have spent the budget; a vote with missing
		// samples is not a vote, so the candidate fails rather than deciding on fewer.
		if r.spent() {
			if i == 0 {
				j.notAsked = true
			} else {
				j.err = errors.New("not all samples sent: " + r.cappedReason())
			}
			return
		}
		calls, reported = 0, false
		resp, err := r.cl.Classify(ctx, req)
		if calls == 0 && !(err != nil && errors.Is(err, sdk.ErrBudget)) {
			// A backend that does not call Start: count the call it made.
			count()
		}
		if !reported {
			// A backend that does not report per call: account what it returned, even with an error.
			req.Used(resp.Usage)
		}
		if err != nil {
			if errors.Is(err, sdk.ErrBudget) && i == 0 && calls == 0 {
				j.notAsked = true
				return
			}
			j.err = fmt.Errorf("classifier %s: %w", r.clName, err)
			return
		}
		if _, err := classify.Check(req.Questions, resp); err != nil {
			j.err = fmt.Errorf("classifier %s: invalid response: %w", r.clName, err)
			return
		}
		bd.Samples = append(bd.Samples, resp.Answers)
	}
	if r.cache != nil && j.key != "" {
		if err := r.cache.Put(j.key, bd); err != nil {
			fmt.Fprintf(r.log, "lintuition: answer cache: %v\n", err)
		}
	}
	r.decide(j, req, bd.Samples)
}

// decide votes over the samples and lets the rule decide; samples that disagree abstain.
func (r *runner) decide(j *job, req sdk.Request, samples [][]sdk.Answer) {
	checked := make([]map[string]sdk.Answer, 0, len(samples))
	for _, s := range samples {
		m, err := classify.Check(req.Questions, sdk.Response{Answers: s})
		if err != nil {
			j.err = fmt.Errorf("cached answers are invalid: %w", err)
			return
		}
		checked = append(checked, m)
	}
	answers, disagree, err := classify.Vote(req.Questions, checked)
	if err != nil {
		j.err = err
		return
	}
	j.asked, j.samples = true, len(samples)
	j.agreement = agreement(req.Questions, checked)
	j.perSample = perSample(req.Questions, checked)
	if disagree != "" {
		j.decision = sdk.Abstain(disagree)
		return
	}
	j.answers = answers
	j.decision = j.rule.Decide(j.cand, answers)
}

func (r *runner) cappedReason() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.capped
}

func (r *runner) problems() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	if r.capped != "" {
		out = append(out, r.capped)
	}
	if r.unknownCost > 0 {
		out = append(out, fmt.Sprintf("%d call(s) reported no usable cost; the run's cost is a lower bound", r.unknownCost))
	}
	return out
}

func evidence(cl, model, version string, j *job) *report.Evidence {
	ev := &report.Evidence{
		Classifier: cl, Model: model, LinterVersion: version,
		Answers: map[string]string{}, Scores: map[string]float64{},
		Samples: j.samples, Replayed: j.replayed, Agreement: j.agreement, PerSample: j.perSample,
	}
	for id, a := range j.answers {
		if a.ConfidenceMeaning != "" {
			if ev.Support == nil {
				ev.Support, ev.Confidence = map[string]string{}, map[string]float64{}
			}
			ev.Support[id] = string(a.ConfidenceMeaning)
			if a.Confidence != nil {
				ev.Confidence[id] = *a.Confidence
			}
		}
		switch {
		case a.Choice != "":
			ev.Answers[id] = a.Choice
			if p, ok := a.Probability(a.Choice); ok {
				ev.Scores[id] = p
			}
		case a.Yes != nil:
			ev.Scores[id] = *a.Yes
		case a.Score != nil:
			ev.Scores[id] = *a.Score
		}
	}
	return ev
}

// agreement is, per question, how the samples split: "current 2, total 1"; nil for one sample.
func agreement(qs []sdk.Question, samples []map[string]sdk.Answer) map[string]string {
	if len(samples) < 2 {
		return nil
	}
	out := map[string]string{}
	for _, q := range qs {
		count := map[string]int{}
		for _, s := range samples {
			a := s[q.ID]
			switch {
			case a.Choice != "":
				count[a.Choice]++
			case a.Yes != nil && *a.Yes > 0.5:
				count["yes"]++
			case a.Yes != nil:
				count["no"]++
			case a.Score != nil:
				count[strconv.FormatFloat(*a.Score, 'g', -1, 64)]++
			}
		}
		keys := make([]string, 0, len(count))
		for k := range count {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, count[k]))
		}
		out[q.ID] = strings.Join(parts, ", ")
	}
	return out
}

// perSample is, per question, each sample's full answer in sample order, unrounded. Nil for one
// sample, whose answer is the evidence itself.
func perSample(qs []sdk.Question, samples []map[string]sdk.Answer) map[string][]map[string]float64 {
	if len(samples) < 2 {
		return nil
	}
	out := map[string][]map[string]float64{}
	for _, q := range qs {
		for _, s := range samples {
			a := s[q.ID]
			v := map[string]float64{}
			switch {
			case a.Choice != "":
				for k, p := range a.Probabilities {
					v[k] = p
				}
			case a.Yes != nil:
				v["yes"] = *a.Yes
			case a.Score != nil:
				v["score"] = *a.Score
			}
			if a.Confidence != nil {
				v["confidence"] = *a.Confidence
			}
			out[q.ID] = append(out[q.ID], v)
		}
	}
	return out
}

// model is the backend's public model identity for evidence, without any account scope.
func (r *runner) model() string {
	if m, ok := r.cl.(interface{ Model() string }); ok {
		return m.Model()
	}
	return ""
}

func hasStatus(ss []report.LinterStatus, name string) bool {
	for _, s := range ss {
		if s.Name == name {
			return true
		}
	}
	return false
}

// relBase is the directory report paths are relative to, per run.relative-path-mode.
func relBase(c *config.Config, dir string) (string, error) {
	wd := dir
	if wd == "" {
		var err error
		if wd, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	wd, _ = filepath.Abs(wd)
	switch c.Run.RelativePathMode {
	case "cfg":
		if c.Path != "" {
			return filepath.Dir(c.Path), nil
		}
	case "gomod":
		if d, ok := upward(wd, "go.mod"); ok {
			return d, nil
		}
	case "gitroot":
		if d, ok := upward(wd, ".git"); ok {
			return d, nil
		}
	}
	return wd, nil
}

func upward(dir, name string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir, true
		}
		p := filepath.Dir(dir)
		if p == dir {
			return "", false
		}
		dir = p
	}
}

func rel(base, path string) string {
	if r, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}

// stable refuses a question whose text differs between candidates of one linter. Question text is
// a fixed template that refers to state fields by name; text that varies carries candidate data,
// which belongs in the state, where the classifier reads it as data.
func (r *runner) stable(linter string, qs []sdk.Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.texts == nil {
		r.texts = map[string]string{}
	}
	for _, q := range qs {
		k := linter + "\x00" + q.ID
		if prev, ok := r.texts[k]; ok && prev != q.Text {
			return fmt.Errorf("the text of question %q varies between candidates; put candidate data in the state", q.ID)
		}
		r.texts[k] = q.Text
	}
	return nil
}

// fingerprint identifies a finding without its line, so it survives code moving around it.
func fingerprint(linter, file, subject, text string, occurrence int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", linter, file, subject, text, occurrence)))
	return hex.EncodeToString(sum[:8])
}
