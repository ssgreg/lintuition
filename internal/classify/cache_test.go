package classify

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ssgreg/lintuition/sdk"
)

func f(v float64) *float64 { return &v }

var q = []sdk.Question{
	{ID: "k", Kind: sdk.Choice, Text: "?", Options: []sdk.Option{{Key: "a"}, {Key: "b"}}},
	{ID: "y", Kind: sdk.Noul, Text: "?"},
	{ID: "s", Kind: sdk.Score, Text: "?", Levels: []string{"0", "1", "2"}},
}

func TestKey(t *testing.T) {
	req := sdk.Request{Linter: "x", State: map[string]any{"help": "h"}, Questions: q}
	base, _ := Key("jev|e|m1", "l@1", 1, req)
	for name, k := range map[string]func() (string, error){
		"model":  func() (string, error) { return Key("jev|e|m2", "l@1", 1, req) },
		"linter": func() (string, error) { return Key("jev|e|m1", "l@2", 1, req) },
		"votes":  func() (string, error) { return Key("jev|e|m1", "l@1", 3, req) },
		"state": func() (string, error) {
			return Key("jev|e|m1", "l@1", 1, sdk.Request{State: map[string]any{"help": "H"}, Questions: q})
		},
		"options": func() (string, error) {
			q2 := append([]sdk.Question(nil), q...)
			q2[0].Options = q2[0].Options[:1]
			return Key("jev|e|m1", "l@1", 1, sdk.Request{State: req.State, Questions: q2})
		},
	} {
		got, err := k()
		if err != nil || got == base {
			t.Errorf("%s must change the key", name)
		}
	}
	again, _ := Key("jev|e|m1", "l@1", 1, sdk.Request{Linter: "other", State: map[string]any{"help": "h"}, Questions: q})
	if again != base {
		t.Error("the same content must give the same key")
	}
}

func TestCache(t *testing.T) {
	now := time.Unix(1000, 0)
	c := &Cache{Dir: filepath.Join(t.TempDir(), "c"), TTL: time.Hour, Now: func() time.Time { return now }}
	key, _ := Key("id", "l@1", 1, sdk.Request{Questions: q})
	if _, ok := c.Get(key); ok {
		t.Fatal("empty cache hit")
	}
	bd := Bundle{Samples: [][]sdk.Answer{{{QuestionID: "k", Choice: "a"}}}}
	if err := c.Put(key, bd); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get(key)
	if !ok || got.Samples[0][0].Choice != "a" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if st, _ := os.Stat(c.path(key)); st.Mode().Perm() != 0o600 {
		t.Errorf("cache file mode %v, want 0600", st.Mode().Perm())
	}
	now = now.Add(2 * time.Hour)
	if _, ok := c.Get(key); ok {
		t.Error("an expired record must miss")
	}
	os.WriteFile(c.path(key), []byte("{corrupt"), 0o600)
	if _, ok := c.Get(key); ok {
		t.Error("a corrupt record must miss")
	}
	if err := c.Put(key, Bundle{}); err == nil {
		t.Error("an empty bundle must not be cached")
	}
	if _, err := (&Cache{}).Clean(false); err == nil {
		t.Error("cleaning without a directory must be refused")
	}
}

func choice(k string, pa float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"k": {QuestionID: "k", Choice: k, Probabilities: map[string]float64{"a": pa, "b": 1 - pa}},
		"y": {QuestionID: "y", Yes: f(0.8)},
		"s": {QuestionID: "s", Score: f(1)},
	}
}

func TestVote(t *testing.T) {
	// Majority with the mean probability over all samples.
	got, why, err := Vote(q, []map[string]sdk.Answer{choice("a", 0.9), choice("a", 0.7), choice("b", 0.2)})
	if err != nil || why != "" || got["k"].Choice != "a" {
		t.Fatalf("majority: %+v %q %v", got, why, err)
	}
	if p := got["k"].Probabilities["a"]; p < 0.599 || p > 0.601 {
		t.Errorf("mean probability of a = %v, want 0.6", p)
	}
	// No strict majority: abstain with the tally.
	three := []map[string]sdk.Answer{choice("a", 0.9), choice("b", 0.1), choice("unclear", 0.5)}
	if _, why, _ := Vote(q, three); why == "" {
		t.Error("a three-way split must disagree")
	}
	// Noul samples on both sides of 0.5 disagree.
	split := []map[string]sdk.Answer{choice("a", 0.9), choice("a", 0.9), choice("a", 0.9)}
	split[1]["y"] = sdk.Answer{QuestionID: "y", Yes: f(0.2)}
	if _, why, _ := Vote(q, split); why == "" {
		t.Error("noul samples on both sides must disagree")
	}
	// Score is the median.
	sc := []map[string]sdk.Answer{choice("a", 0.9), choice("a", 0.9), choice("a", 0.9)}
	sc[0]["s"], sc[1]["s"], sc[2]["s"] = sdk.Answer{QuestionID: "s", Score: f(0)}, sdk.Answer{QuestionID: "s", Score: f(2)}, sdk.Answer{QuestionID: "s", Score: f(1.5)}
	if got, _, _ := Vote(q, sc); *got["s"].Score != 1.5 {
		t.Errorf("median score %v, want 1.5", *got["s"].Score)
	}
	if _, _, err := Vote(q, nil); err == nil {
		t.Error("no samples is an error")
	}
}

func TestCleanRemovesOnlyRecords(t *testing.T) {
	dir := t.TempDir()
	foreign := []string{"README.md", "src/main.go", "ab/notes.txt", "zz/0000.json"}
	for _, f := range foreign {
		p := filepath.Join(dir, f)
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte("keep"), 0o600)
	}
	c := &Cache{Dir: dir, TTL: time.Hour}
	key, _ := Key("id", "l@1", 1, sdk.Request{Questions: q})
	c.Put(key, Bundle{Samples: [][]sdk.Answer{{{QuestionID: "k", Choice: "a"}}}})
	n, err := c.Clean(false)
	if err != nil || n != 1 {
		t.Fatalf("removed %d, %v", n, err)
	}
	for _, f := range foreign {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s was removed", f)
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the configured directory itself must stay")
	}
}

func TestLookupValidates(t *testing.T) {
	c := &Cache{Dir: t.TempDir(), TTL: time.Hour}
	key, _ := Key("id", "l@1", 3, sdk.Request{Questions: q})
	good := []sdk.Answer{{QuestionID: "k", Choice: "a"}, {QuestionID: "y", Yes: f(0.5)}, {QuestionID: "s", Score: f(1)}}
	c.Put(key, Bundle{Samples: [][]sdk.Answer{good}})
	if _, ok := c.Lookup(key, 3, q); ok {
		t.Error("one sample under a three-vote key must miss")
	}
	bad := []sdk.Answer{{QuestionID: "k", Choice: "zzz"}, good[1], good[2]}
	c.Put(key, Bundle{Samples: [][]sdk.Answer{good, good, bad}})
	if _, ok := c.Lookup(key, 3, q); ok {
		t.Error("an invalid sample must miss")
	}
	c.Put(key, Bundle{Samples: [][]sdk.Answer{good, good, good}})
	if _, ok := c.Lookup(key, 3, q); !ok {
		t.Error("a valid record must hit")
	}
}

func TestVoteKeepsConfidence(t *testing.T) {
	qs := []sdk.Question{{ID: "k", Kind: sdk.Choice, Text: "?", Options: []sdk.Option{{Key: "a"}, {Key: "b"}}}}
	s := func(c float64) map[string]sdk.Answer {
		return map[string]sdk.Answer{"k": {QuestionID: "k", Choice: "a", Probabilities: map[string]float64{"a": 1}, Confidence: f(c), ConfidenceMeaning: sdk.ConfidenceOptionMass}}
	}
	got, _, _ := Vote(qs, []map[string]sdk.Answer{s(0.8), s(0.8), s(0.8)})
	if got["k"].Confidence == nil || *got["k"].Confidence != 0.8000000000000002 && *got["k"].Confidence != 0.8 || got["k"].ConfidenceMeaning != sdk.ConfidenceOptionMass {
		t.Fatalf("vote must keep the option mass: %+v", got["k"])
	}
}
