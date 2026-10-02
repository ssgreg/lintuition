package c

import (
	"errors"
	"fmt"
	"runtime"
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

func TestValidateRejectsStop30(t *testing.T) { // 30 unsupported: t.Skip ends the test first
	err := Validate("")
	if err != nil {
		t.Skip("unsupported by this backend")
		t.Fatal(err)
	}
}

func TestValidateRejectsStop31(t *testing.T) { // 31 unsupported: t.SkipNow ends the test first
	err := Validate("")
	if err != nil {
		t.SkipNow()
		t.Fatal(err)
	}
}

func TestValidateRejectsStop32(t *testing.T) { // 32 unsupported: panic ends the test first
	err := Validate("")
	if err != nil {
		panic(err)
		t.Fatal(err)
	}
}

func TestValidateRejectsStop33(t *testing.T) { // 33 unsupported: runtime.Goexit ends the goroutine first
	err := Validate("")
	if err != nil {
		runtime.Goexit()
		t.Fatal(err)
	}
}

func TestValidateRejectsStop34(t *testing.T) { // 34 unsupported: a helper may end the test first
	err := Validate("")
	if err != nil {
		helper(t)
		t.Fatal(err)
	}
}

func TestValidateRejectsStop35(t *testing.T) { // 35 unsupported: a log whose argument calls may not return
	err := Validate("")
	if err != nil {
		t.Log(fmt.Sprint(err))
		t.Fatal(err)
	}
}

func helper(t *testing.T) { t.Skip("skip") }

func skipMessage(t *testing.T) string {
	t.Skip("backend unavailable")
	return "unreachable"
}

func skipT(t *testing.T) *testing.T {
	t.Skip("backend unavailable")
	return t
}

func TestValidateRejectsStop36(t *testing.T) { // 36 unsupported: the failure's argument may skip the test
	err := Validate("")
	if err != nil {
		t.Fatal(skipMessage(t))
	}
}

func TestValidateRejectsStop37(t *testing.T) { // 37 unsupported: the failure's receiver may skip the test
	err := Validate("")
	if err != nil {
		skipT(t).Fatal(err)
	}
}

func TestValidateRejectsStop38(t *testing.T) { // 38 unsupported: the log's receiver may skip the test
	err := Validate("")
	if err != nil {
		skipT(t).Log(err)
		t.Fatal(err)
	}
}

func TestValidateAcceptsConverted(t *testing.T) { // 39 candidate: a conversion in the argument is not a call
	err := Validate("x")
	if err != nil {
		t.Log(any(err))
		t.Fatal(error(err))
	}
}

func errFn() error { return ErrEmpty }

func TestValidateAcceptsErrorText(t *testing.T) { // 40 candidate: err.Error() on the asserted error
	err := Validate("x")
	if err != nil {
		t.Log(err.Error())
		t.Fatal(err.Error())
	}
}

func TestValidateAcceptsOtherText(t *testing.T) { // 41 unsupported: Error() on a different error
	other := ErrEmpty
	err := Validate("x")
	if err != nil {
		t.Fatal(other.Error())
	}
}

func TestValidateAcceptsCallText(t *testing.T) { // 42 unsupported: Error() on a call result
	err := Validate("x")
	if err != nil {
		t.Fatal(errFn().Error())
	}
}

// TestValidateAcceptsDocumented checks that a plain token passes; TestValidateAcceptsDocumentedToo
// is a different name and stays.
func TestValidateAcceptsDocumented(t *testing.T) { // 43 candidate: the doc is sent, the test's own name masked
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
}

// Every mention is masked: see TestValidateAcceptsSpaced (TestValidateAcceptsSpaced).
func TestValidateAcceptsSpaced(t *testing.T) { // 44 candidate: every mention of the name is masked
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
}

func TestWrapNilError(t *testing.T) { // 45 candidate: a nil error passed to an error parameter
	var err error
	if got := Wrap(err, "p"); got != nil {
		t.Fatal(got)
	}
}

func TestJoinNothing(t *testing.T) { // 46 candidate: ...error given no argument passes no error
	if err := Join(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectEmpty(t *testing.T) { // 47 candidate: a []error parameter is not an error
	if err := Collect(nil); err != nil {
		t.Fatal(err)
	}
}

func TestStore_RestoreKeepsCause(t *testing.T) { // 48 candidate: a method that takes an error
	var s Store
	if err := s.Restore(ErrEmpty); err == nil {
		t.Fatal("no error")
	}
}

// A note for the reader of this file, kept apart by a blank line.

func TestValidateAcceptsUndocumented(t *testing.T) { // 49 candidate: a comment above a blank line is not the doc
	if err := Validate("x"); err != nil {
		t.Fatal(err)
	}
}

func TestJoinOne(t *testing.T) { // 50 candidate: an argument to ...error is an error value
	if err := Join(ErrEmpty); err != nil {
		t.Fatal(err)
	}
}

func TestJoinSpread(t *testing.T) { // 51 candidate: a slice spread into ...error is not an error value
	errs := []error{ErrEmpty}
	if err := Join(errs...); err != nil {
		t.Fatal(err)
	}
}

func TestStore_JoinThroughExpression(t *testing.T) { // 52 candidate: a method expression's receiver is not an error argument
	var s Store
	if err := Store.Join(s); err != nil {
		t.Fatal(err)
	}
}

func TestStore_JoinThroughExpressionOne(t *testing.T) { // 53 candidate: a method expression with one error
	var s Store
	if err := Store.Join(s, ErrEmpty); err != nil {
		t.Fatal(err)
	}
}

func TestStore_JoinThroughExpressionSpread(t *testing.T) { // 54 candidate: a method expression with a spread slice
	var s Store
	errs := []error{ErrEmpty}
	if err := Store.Join(s, errs...); err != nil {
		t.Fatal(err)
	}
}

func TestStore_PrefixThroughExpression(t *testing.T) { // 55 candidate: the receiver and the string take the first two parameters
	s := &Store{}
	if err := (*Store).Prefix(s, "p"); err != nil {
		t.Fatal(err)
	}
}

func TestStore_PrefixDirectOne(t *testing.T) { // 56 candidate: a direct method call with one variadic error
	var s Store
	if err := s.Prefix("p", ErrEmpty); err != nil {
		t.Fatal(err)
	}
}

func TestUnwrapExplicit(t *testing.T) { // 57 candidate: an explicit instantiation with error
	if err := Unwrap[error](nil); err != nil {
		t.Fatal(err)
	}
}

func TestUnwrapInferred(t *testing.T) { // 58 candidate: an inferred instantiation with error
	if err := Unwrap(ErrEmpty); err == nil {
		t.Fatal("no error")
	}
}

func TestConsumeForwarded(t *testing.T) { // 59 candidate: a forwarded result pair supplies the error
	if err := Consume(Pair()); err != nil {
		t.Fatal(err)
	}
}

func TestTallyForwarded(t *testing.T) { // 60 candidate: a forwarded result pair without an error
	if err := Tally(Count()); err != nil {
		t.Fatal(err)
	}
}

func TestAliasNilError(t *testing.T) { // 61 candidate: an alias of error is an error parameter
	if err := Alias(nil); err != nil {
		t.Fatal(err)
	}
}

func TestWrapAVeryLongFunctionNameThatGoesOnAndOnAndOnUntilItIsLongerThanAnyOrdinaryNameWouldBeNilError(t *testing.T) { // 62 candidate: a long name keeps the facts structural
	if err := WrapAVeryLongFunctionNameThatGoesOnAndOnAndOnUntilItIsLongerThanAnyOrdinaryNameWouldBe(nil); err != nil {
		t.Fatal(err)
	}
}
