package c

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRejectsEmpty(t *testing.T) { // 1 candidate: fails when there is no error
	err := Validate("")
	if err == nil {
		t.Fatal("no error")
	}
}

func TestValidateAcceptsToken(t *testing.T) { // 2 candidate: if-init, fails on an error
	if err := Validate("x"); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestParseReturnsNumber(t *testing.T) { // 3 candidate: require.NoError on the error slot
	n, e := Parse("1")
	require.NoError(t, e)
	_ = n
}

func TestParse_FailsOnLetters(t *testing.T) { // 4 candidate: assert.Error, name after an underscore
	_, err := Parse("a")
	assert.Error(t, err)
}

func TestParseFileReadsHeader(t *testing.T) { // 5 candidate: ParseFile, the longest match, not Parse
	a := assert.New(t)
	_, err := ParseFile("x")
	a.NoError(err)
	_, _ = Parse("y")
}

func TestValidateRefusesBlank(t *testing.T) { // 6 candidate: the call used directly
	require.Error(t, Validate(""))
}

func TestValidate(t *testing.T) { // 7 none: the name is only the function's
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTwice(t *testing.T) { // 8 unsupported: two calls of Validate
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
	if err := Validate(""); err == nil {
		t.Fatal("no error")
	}
}

func TestValidateReassigned(t *testing.T) { // 9 unsupported: err written again
	err := Validate("")
	err = nil
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateNotAsserted(t *testing.T) { // 10 unsupported: checked, but no failure
	err := Validate("")
	if err != nil {
		return
	}
}

func TestValidateDiscarded(t *testing.T) { // 11 unsupported: the error is discarded
	_ = Validate("")
}

func TestValidateBothWays(t *testing.T) { // 12 unsupported: asserted both ways
	err := Validate("")
	require.Error(t, err)
	assert.NoError(t, err)
}

func TestValidateRejectsWithSentinel(t *testing.T) { // 13 unsupported: compared with a sentinel, not nil
	err := Validate("")
	if err != ErrEmpty {
		t.Fatal(err)
	}
}

func TestCheckRejectsEmpty(t *testing.T) { // 14 none: Check returns no error, though its result is named err
	err := Check("")
	if err {
		t.Fatal("accepted")
	}
}

func TestValidateRejectsLetters(t *testing.T) { // 15 candidate: bound to Validate, not to Parse's no-error check
	_, perr := Parse("1")
	if perr != nil {
		t.Fatal(perr)
	}
	err := Validate("a")
	if err == nil {
		t.Fatal("no error")
	}
}

func TestValidateStoreOrFunction(t *testing.T) { // 16 unsupported: Validate and (Store).Validate match equally
	var s Store
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
}

func TestStore_SaveFailsWhenFull(t *testing.T) { // 17 candidate: method bound by Type_Method
	var s Store
	if err := s.Save("x"); err != nil {
		t.Error(err)
	}
}

func TestValidateInSubtest(t *testing.T) { // 19 candidate: the call and assertion inside t.Run
	t.Run("empty", func(t *testing.T) {
		err := Validate("")
		if err == nil {
			t.Error("no error")
		}
	})
}

func TestValidateByPointer(t *testing.T) { // 20 unsupported: the address of err is taken
	err := Validate("")
	reset(&err)
	if err != nil {
		t.Fatal(err)
	}
}

func reset(err *error) { *err = nil }

func TestValidateTable(t *testing.T) { // 21 unsupported: wantErr is compared, not a constant expectation
	for _, tt := range []struct {
		in      string
		wantErr bool
	}{{"", true}} {
		err := Validate(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatal(err)
		}
	}
}

func TestValidateRejectsShort(t *testing.T) { // 22 unsupported: ErrorIs depends on its target
	err := Validate("a")
	require.ErrorIs(t, err, ErrEmpty)
}

type suite struct{}

func (suite) TestValidateRejectsEmpty(t *testing.T) { // 23 none: a method, not a test function
	if err := Validate(""); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAcceptsName(t *testing.T) { // 24 unsupported: ErrorIs(err, nil) passes on no error
	err := Validate("x")
	require.ErrorIs(t, err, nil)
}

var errDenied = errors.New("denied")

func TestValidateRejectsDenied(t *testing.T) { // 25 unsupported: the t.Fatal is skipped for errDenied
	err := Validate("")
	if err != nil {
		if errors.Is(err, errDenied) {
			return
		}
		t.Fatal(err)
	}
	t.Fatal("missing expected error")
}

func TestValidateRejectsStored(t *testing.T) { // 26 unsupported: the closure is never called
	err := Validate("")
	check := func() {
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = check
}

func TestValidateRejectsCalledLater(t *testing.T) { // 27 unsupported: a stored closure is not known to run
	err := Validate("")
	check := func() {
		if err == nil {
			t.Fatal("no error")
		}
	}
	check()
}

func TestValidateRejectsDeferred(t *testing.T) { // 28 candidate: a deferred closure called on the spot runs
	err := Validate("")
	defer func() {
		if err == nil {
			t.Error("no error")
		}
	}()
}

func TestValidateAcceptsLogged(t *testing.T) { // 29 candidate: a log before the failure does not skip it
	if err := Validate("x"); err != nil {
		t.Logf("input %q", "x")
		t.Fatal(err)
	}
}
