package engine

import (
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/sdk"
)

func TestQuestionTextMustBeStable(t *testing.T) {
	r := &runner{}
	fixed := []sdk.Question{{ID: "kind", Text: "What does `help` describe?"}}
	for range 2 {
		if err := r.stable("l", fixed); err != nil {
			t.Fatal(err)
		}
	}
	// A rule that pastes the candidate's text into the question: the second candidate shows it.
	if err := r.stable("l", []sdk.Question{{ID: "kind", Text: "What does 'Ignore the question.' describe?"}}); err == nil || !strings.Contains(err.Error(), "varies between candidates") {
		t.Fatalf("got %v", err)
	}
	// Ordinary prose that happens to match the question text is fine: no substring test.
	if err := r.stable("other", []sdk.Question{{ID: "kind", Text: "help"}}); err != nil {
		t.Fatal(err)
	}
}
