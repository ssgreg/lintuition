package docsignature

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/internal/classify"
	"github.com/ssgreg/lintuition/sdk"
)

// caseNo reads the "// N" case number from the candidate's source line.
func caseNo(t *testing.T, pos token.Position) string {
	b, err := os.ReadFile(pos.Filename)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(b), "\n")[pos.Line-1]
	i := strings.LastIndex(line, "// ")
	if i < 0 {
		return "?"
	}
	return strings.Fields(line[i+3:])[0]
}

func num(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var cs []*sdk.Candidate
	seen := map[string]bool{}
	for _, r := range res {
		for _, c := range r.Result.([]*sdk.Candidate) {
			// The test variant of the package repeats the non-test files.
			k := c.Pos.String()
			if seen[k] {
				continue
			}
			seen[k] = true
			cs = append(cs, c)
		}
	}
	sort.SliceStable(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s sends more than the doc: facts %v, source %v", n, c.Payload.Facts, c.Payload.Source)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported")
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Local["claim"], c.Payload.Prose["doc"]))
	}
	want := []string{
		`1 result "the documented function writes the buffer to disk and returns the number of bytes written."`,
		`2 result "the documented function reports whether the buffer is non-empty."`,
		`3 result "the documented function blocks and returns when ctx is done."`,
		`7 result "the documented function returns the size; see GetSizeLimit."`,
		`8 error "the documented function returns an error if the name is empty."`,
		`9 error "the documented function returns ErrNotFound when the key is missing."`,
		`14 unsupported`,
		`15 unsupported`,
		`16 unsupported`,
		`17 unsupported`,
		`18 unsupported`,
		`19 error "the documented function returns an error description."`,
		`20 result "the documented function writes pending data and returns the count."`,
		`21 error "the documented function returns the value or an error."`,
		`26 error "the documented function logs an error when the counter overflows."`,
		`33 result "the documented function returns the fixture path."`,
		`40 unsupported`,
		`41 unsupported`,
		`42 unsupported`,
		`43 unsupported`,
		`44 unsupported`,
		`45 unsupported`,
		`46 unsupported`,
		`47 unsupported`,
		`48 unsupported`,
		`49 result "the documented function returns the number of bytes written."`,
		`50 result "the documented function returns the number of bytes written."`,
		`53 result "the documented function returns the number of bytes written."`,
		`54 unsupported`,
		`55 unsupported`,
		`56 unsupported`,
		`58 error "the documented function returns an error."`,
		`59 error "the documented function returns the chain or an error."`,
		`60 result "the documented function returns the number of completed operations."`,
		`63 result "the documented function returns a value."`,
		`64 unsupported`,
		`65 unsupported`,
		`66 unsupported`,
		`67 error "the documented function returns an error value."`,
		`68 unsupported`,
		`69 unsupported`,
		`70 unsupported`,
		`71 unsupported`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func yes(id string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{id: {QuestionID: id, Yes: &p}}
}

// both answers the two questions of a candidate with an output: the error one, then the other.
func both(errYes, valueYes float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		questionWriterError: {QuestionID: questionWriterError, Yes: &errYes},
		questionWriter:      {QuestionID: questionWriter, Yes: &valueYes},
	}
}

func TestQuestions(t *testing.T) {
	r := &rule{threshold: 0.85}
	for claim, id := range map[string]string{claimResult: "returns_result", claimError: "returns_error"} {
		qs := r.Questions(&sdk.Candidate{Local: map[string]string{"claim": claim}})
		if len(qs) != 1 || qs[0].ID != id || qs[0].Kind != sdk.Noul || !strings.Contains(qs[0].Text, "`doc`") {
			t.Errorf("claim %s: %+v", claim, qs)
		}
		if err := qs[0].Validate(); err != nil {
			t.Errorf("claim %s: %v", claim, err)
		}
	}
	// The writer fact switches the result claim to its own question, and only the result claim.
	qs := r.Questions(&sdk.Candidate{Local: map[string]string{"claim": claimResult, "writer": "parameter w is an io.Writer it could write to"}})
	if len(qs) != 2 || qs[0].ID != questionWriterError || qs[1].ID != questionWriter || !strings.Contains(qs[1].Text, "`writer`") {
		t.Errorf("writer: %+v", qs)
	}
	for _, q := range qs {
		if err := q.Validate(); err != nil || q.Kind != sdk.Noul || !strings.Contains(q.Text, "`doc`") {
			t.Errorf("writer: %s: %v", q.ID, err)
		}
	}
	if qs := r.Questions(&sdk.Candidate{Local: map[string]string{"claim": claimError, "writer": "x"}}); qs[0].ID != "returns_error" {
		t.Errorf("error claim with a writer: %+v", qs)
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	result := &sdk.Candidate{Local: map[string]string{"name": "Sync", "claim": claimResult}}
	errc := &sdk.Candidate{Local: map[string]string{"name": "Validate", "claim": claimError}}
	writer := &sdk.Candidate{Local: map[string]string{"name": "Proxy", "claim": claimResult, "writer": "parameter w is an http.ResponseWriter it could write to"}}
	for _, tc := range []struct {
		name            string
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"result: promised", result, yes("returns_result", 0.95), true, false},
		{"result: at the threshold", result, yes("returns_result", 0.85), true, false},
		{"result: not promised", result, yes("returns_result", 0.05), false, false},
		{"result: no at the threshold", result, yes("returns_result", 0.15), false, false},
		{"result: weak yes abstains", result, yes("returns_result", 0.7), false, true},
		{"result: weak no abstains", result, yes("returns_result", 0.3), false, true},
		{"result: no probability", result, map[string]sdk.Answer{"returns_result": {QuestionID: "returns_result"}}, false, true},
		{"result: the other question's answer is ignored", result, yes("returns_error", 0.99), false, true},
		{"error: promised", errc, yes("returns_error", 0.9), true, false},
		{"error: not promised", errc, yes("returns_error", 0.1), false, false},
		{"error: weak abstains", errc, yes("returns_error", 0.5), false, true},
		{"error: missing answer", errc, map[string]sdk.Answer{}, false, true},
		{"writer: a value promised to the caller", writer, both(0.05, 0.9), true, false},
		{"writer: an error promised to the caller", writer, both(0.9, 0.05), true, false},
		{"writer: an error, the rest weak", writer, both(0.9, 0.5), true, false},
		{"writer: what goes to the client", writer, both(0.05, 0.05), false, false},
		{"writer: both at the no threshold", writer, both(0.15, 0.15), false, false},
		{"writer: weak value abstains", writer, both(0.05, 0.5), false, true},
		{"writer: weak error abstains", writer, both(0.5, 0.05), false, true},
		{"writer: one answer missing", writer, yes(questionWriter, 0.05), false, true},
		{"writer: the plain result question is ignored", writer, yes("returns_result", 0.99), false, true},
		{"result: the writer questions are ignored", result, both(0.99, 0.99), false, true},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(result, yes("returns_result", 0.9)); d.Message != "doc of Sync says it returns a result, but Sync returns nothing" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(writer, both(0.05, 0.9)); d.Message != "doc of Proxy says it returns a result, but Proxy returns nothing" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(writer, both(0.9, 0.9)); d.Message != "doc of Proxy says it returns an error, but Proxy returns nothing" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(errc, yes("returns_error", 0.9)); d.Message != "doc of Validate says it returns an error, but Validate has no error result" {
		t.Errorf("message: %s", d.Message)
	}
}

func TestWords(t *testing.T) {
	for _, tc := range []struct {
		text          string
		ret, errWords bool
	}{
		{"Run returns when done.", true, false},
		{"Valid Reports Whether x.", true, false},
		{"Next yields the next item.", true, false},
		{"The returned value is cached.", true, false},
		{"Returning early is fine.", true, false},
		{"Lookup gives back the value.", true, false},
		{"Pop hands back the head.", true, false},
		{"Evict tells the caller whether it was present.", true, false},
		{"Has tells callers if the key exists.", true, false},
		{"Check reports if the name is valid.", true, false},
		{"It reports progress to the logger.", false, false},
		{"It tells the user to retry.", false, false},
		{"Close closes it.", false, false},
		{"It logs an error.", false, true},
		{"Errors are collected.", false, true},
		{"err is always nil.", false, true},
		{"See ErrClosed.", false, true},
		{"See errNoRows.", false, true},
		{"Uses errgroup.", false, false},
		{"Errorf builds a message.", false, false},
		{"Prints an errno.", false, false},
		{"A returnable value.", false, false},
	} {
		if got := returnWords.MatchString(tc.text); got != tc.ret {
			t.Errorf("return words in %q: %v", tc.text, got)
		}
		if got := errorWords.MatchString(tc.text); got != tc.errWords {
			t.Errorf("error words in %q: %v", tc.text, got)
		}
	}
}

// TestFixtureDocsCarryNoHints checks that the twins' and the showcase's explanations ("Defect: ...",
// "doc-vs-signature: ...") are not part of any doc this linter sends: they would tell the classifier
// the answer and make a live evaluation measure the hint instead of the doc.
func TestFixtureDocsCarryNoHints(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/twins/docsignature/*.go")
	files = append(files, "../../examples/showcase/errors.go")
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		check := func(doc *ast.CommentGroup, id *ast.Ident) {
			if doc == nil {
				return
			}
			for _, hint := range []string{"Defect", "Fixed twin", "Negative", "doc-vs-signature", "want"} {
				if strings.Contains(doc.Text(), hint) {
					t.Errorf("%s: the doc of %s carries %q", name, id.Name, hint)
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				check(n.Doc, n.Name)
			case *ast.InterfaceType:
				for _, m := range n.Methods.List {
					if len(m.Names) == 1 {
						check(m.Doc, m.Names[0])
					}
				}
			}
			return true
		})
	}
}

func TestWriterExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "w")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.SliceStable(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 || c.Unsupported != "" {
			t.Errorf("case %s: source %v, unsupported %q", n, c.Payload.Source, c.Unsupported)
		}
		w, _ := c.Payload.Facts["writer"].(string)
		if w != c.Local["writer"] || len(c.Payload.Facts) > 1 || (w == "") != (len(c.Payload.Facts) == 0) {
			t.Errorf("case %s: facts %v, local writer %q", n, c.Payload.Facts, c.Local["writer"])
		}
		if _, err := classify.State(c.Payload, classify.Prose); err != nil {
			t.Errorf("case %s: the payload is refused: %v", n, err)
		}
		if w == "" {
			got = append(got, n+" "+c.Local["claim"])
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Local["claim"], w))
	}
	want := []string{
		`1 result "parameter w is an http.ResponseWriter it could write a response to"`,
		`2 result`,
		`3 result`,
		`4 result`,
		`5 result`,
		`6 result "receiver r gives access to an http.ResponseWriter at r.w"`,
		`7 result "parameter c gives access to an http.ResponseWriter at c.Writer"`,
		`8 result "parameter o gives access to an http.ResponseWriter at o.in.resp.w"`,
		`9 result "parameter c gives access to an http.ResponseWriter at c.Response()"`,
		`10 result "parameter 1 is an http.ResponseWriter it could write a response to"`,
		`11 result`,
		`12 result`,
		`14 result`,
		`15 result`,
		`16 result`,
		`17 result`,
		`18 result`,
		`19 result`,
		`20 result`,
		`21 result "parameter a gives access to an http.ResponseWriter at a.in.resp.w"`,
		`22 result "parameter s gives access to an http.ResponseWriter at s.Short"`,
		`23 result`,
		`24 result`,
		`25 result`,
		`26 result`,
		`27 result`,
		`28 result`,
		`29 result`,
		`30 result`,
		`31 result`,
		`32 result`,
		`33 result "parameter b gives access to an http.ResponseWriter at b.w"`,
		`34 result "parameter w is an http.ResponseWriter it could write a response to"`,
		`35 result "parameter t gives access to an http.ResponseWriter at t.rw"`,
		`36 result`,
		`37 result "parameter p is an http.ResponseWriter it could write a response to"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestWriterWalkBounds checks that a walk that runs out of nodes finds nothing rather than a writer
// past the bound, and that a writer within it is found.
func TestWriterWalkBounds(t *testing.T) {
	// A stand-in for net/http's ResponseWriter, built on a Header type of a package of that path.
	http := types.NewPackage("net/http", "http")
	header := types.NewNamed(types.NewTypeName(0, http, "Header", nil), types.NewMap(types.Typ[types.String], types.NewSlice(types.Typ[types.String])), nil)
	sig := func(params, results *types.Tuple) *types.Signature {
		return types.NewSignatureType(nil, nil, nil, params, results, false)
	}
	rw := types.NewInterfaceType([]*types.Func{
		types.NewFunc(0, http, "Header", sig(nil, types.NewTuple(types.NewVar(0, nil, "", header)))),
		ioWriter.Method(0),
		types.NewFunc(0, http, "WriteHeader", sig(types.NewTuple(types.NewVar(0, nil, "code", types.Typ[types.Int])), nil)),
	}, nil).Complete()
	fields := func(n int) *types.Struct {
		var fs []*types.Var
		for i := 0; i < n; i++ {
			fs = append(fs, types.NewField(0, nil, fmt.Sprintf("F%d", i), types.Typ[types.Int], false))
		}
		fs = append(fs, types.NewField(0, nil, "W", rw, false))
		return types.NewStruct(fs, nil)
	}
	if k, p := findWriter(nil, types.NewStruct([]*types.Var{types.NewField(0, nil, "W", ioWriter, false)}, nil)); k != "" {
		t.Errorf("a plain io.Writer: %q %q", k, p)
	}
	if k, p := findWriter(nil, fields(10)); k != "http.ResponseWriter" || p != "W" {
		t.Errorf("small: %q %q", k, p)
	}
	if k, p := findWriter(nil, fields(maxWriterNodes)); k != "" || p != "" {
		t.Errorf("past the node bound: %q %q", k, p)
	}
}
