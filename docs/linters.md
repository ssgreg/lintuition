# How the linters work

Every lintuition linter catches the same kind of bug: text that a person wrote about the code says
something the code does not do. A counter's Help describes a gauge. A log line says "saved" before the
save. A test named "rejects bad input" fails when the input is rejected. The compiler is happy with
all of these, and so are the usual linters.

This page walks through each linter: what it finds, what it reads from your code, what it sends to the
classifier, and how it decides. Every example below is real code in
[examples/showcase](../examples/showcase), and a test checks that each one is caught. If you only
read one section, read the next one.

## The two halves

Each linter works in two steps, and the split is the whole design.

1. **Go code finds the place and the facts.** An ordinary `go/analysis` analyzer walks your typed
   code and pulls out facts it can establish exactly: this literal is a `prometheus.CounterOpts`, this
   log call is at error level, this test's `want` is compared with the result of `IsExpired`. Facts
   come from `go/types`, never from spelling. A variable named `err` is not an error until its type
   says so.
2. **The classifier reads the human text** (a Help string, a log message, a test name, a comment),
   sometimes with a few facts named by identifiers ("the result of IsExpired"), and answers one or
   two narrow questions about it. "What kind of value does this Help describe: a running total, a
   current value or a distribution?"

Then Go code compares the answer with the facts and decides. The classifier never sees your source
and never decides whether something is a finding. Most facts come from object identity; where a
linter falls back on names (a logger package, a redaction function), its section says so.

Why split it like this? Because a classifier is good at reading what a sentence means and bad at
following code. The prototype measured it: asked in one question whether a test name matched its
assertion, the classifier caught 0 of 3 seeded bugs; split into "what does the name say?" plus a code
check, it caught 3 of 3.

## Three outcomes besides a finding

- **Clean.** No finding under this rule: the classifier answered confidently, and the answer does not
  conflict with the code. That includes text that makes no claim the rule can check (a case name that
  only describes the input, a message that gives no advice). It is not proof that the text is right.
- **Abstained.** The classifier was not confident enough. Every linter has a threshold, and below it
  the linter stays quiet whichever way the answer leans. Every multiple-choice question also has an
  "unclear" option, and choosing it abstains too.
- **Unsupported.** The analyzer saw the shape but could not establish the facts, so it did not ask.
  Examples: a log message built at run time, a test table that is modified inside its loop, two
  calls that could be the one a test is about. These are counted, so a run shows what it could not
  check instead of looking clean.

The run summary prints asked, abstained, unsupported, skipped and failed counts per run (and per
linter in the JSON report); clean is what is asked and neither abstained nor reported. Shapes outside
a linter's scope (a log call with no fields, for log-sensitive-field) are not candidates and are not
counted at all.

Linters err on the side of unsupported. A missed bug costs less than a confident wrong finding that
teaches people to ignore the tool.

## What leaves your machine

The question text is fixed per linter. Candidate data goes in a separate state object: person-written
text as prose, sent as written, and identifiers or short descriptions built from them ("the result of
IsExpired", "value cfg.Password, type string") as facts.

So what leaves is the text the linter is about: a Help string, a log or error message, a printf
format, a comment, a test or case name, a nolint reason. These are string literals in your code, and
they are sent. What the built-in linters never send is Go source, and the value of anything your code logs or
passes around: a logged field is described by where its value comes from, not by its value. A plugin
linter could ask to send source; only `semantic.payload: source` lets that through. `semantic.payload: facts` sends structural facts only and skips every
linter that needs prose; `lintuition run --dry-run --preview requests.jsonl` shows exactly what would
be sent.

## Thresholds

The defaults below come from the prototype and are checked only on this repository's synthetic twins,
not on a labelled set of real code. Scores are not comparable between backends: a 0.8 from one
classifier is not a 0.8 from another, but a rule's threshold stays what you configured whichever
backend you use. Treat findings as review hints, and run `lintuition eval` on your own twins
before trusting a new backend or a new threshold.

Every linter takes a `threshold` setting:

```yaml
linters:
  settings:
    premature-success:
      threshold: 0.7
```

---

## Metrics

### metric-type-vs-help

**Finds:** a Prometheus metric whose Help describes a different kind of value than its type records.

```go
// A counter of seconds, but the Help talks about utilization: dashboards get built wrong.
prom.NewCounter(prom.CounterOpts{Name: "io_seconds_total", Help: "Disk I/O utilization."})
```

**Reads:** `CounterOpts`, `GaugeOpts`, `HistogramOpts` and `SummaryOpts` literals, matched by type
(an alias or a renamed import still counts), and their Help when it is a constant.

**Sends:** the Help text only. The metric name stays local: a name ending in `_total` would tell the
classifier the answer.

**Asks:** what kind of value the Help describes: a running total, a current value or a distribution.

**Decides:** reports when the answer does not fit the type (a counter's Help must describe a running
total, a gauge's a current value, a histogram's or summary's a distribution). Threshold 0.8.

**Unsupported:** a Help built at run time.

promlint checks metric names. This checks what the Help says.

---

## Logs

### premature-success

**Finds:** a log line that reports success before the call that can still fail.

```go
log.Info("config saved to disk") // already "saved"...
if err := store.SaveConfig(cfg); err != nil {
	return err // ...and then the save fails
}
```

**Reads:** a log call followed, in the same block, by a statement that keeps an error result:
`err := f()`, `if err := f(); ...`, `return f()`. The error is found by the result's type, so a
renamed error still counts and an `int` named `err` does not. The message and the callee's name must
share a word ("config" in "config saved" and `SaveConfig`).

**Sends:** the message and the callee's name.

**Asks:** whether the message says the operation completed, is starting, or is only progress.

**Decides:** reports "completed". Threshold 0.6: the one real catch in the prototype scored 0.61 to
0.72.

**Unsupported:** a message built at run time; and any log after an earlier call the message could
also be about, in this function or an enclosing block ("reset a, check, log, reset b"). Which call
the log reports is not something the code can tell.

**Quiet on:** `go f()` (nothing is waited for), a discarded error (`_ = f()`), error-level logs.

### log-sensitive-field

**Finds:** a structured log field that carries a secret: a password, a token, key material.

```go
logger.Info("connecting", slog.String("password", cfg.Password))
```

**Reads:** fields of structured loggers only: constructors that return a field type (`slog.String`,
`zap.String`, `logf.String`), slog's key, value pairs, `slog.Group` and `WithGroup` as prefixed keys,
and fields added along a chain (`slog.With`, logrus `WithField`, zerolog `Str`). `log.Println`'s
arguments are not fields. It skips booleans, times and durations, and results of functions whose name starts
with a redaction verb (`Redact`, `MaskToken`, `HashPassword`); the name is a heuristic, not proof the
function removes the secret.

Literal values are left out too, and that is a coverage limit, not safety: telling a placeholder from
a hard-coded secret would mean sending the literal. A committed secret logged as a literal is missed;
a secret scanner is the tool for those.

**Sends:** for each field, the key, where the value comes from and its type: "key password, value
cfg.Password, type string". Never the value. Plus the message, when it is constant.

**Asks:** what the most sensitive value is: an authentication secret, personal information, or a
public identifier (the name or id of a secret is metadata, not a secret).

**Decides:** reports "secret". Threshold 0.85. In the prototype this found a password and a JWT
logged as is.

**Unsupported:** fields it cannot read (a dynamic key, a group with a dynamic name); these are
counted next to the readable ones, so a partly read call does not look fully checked. A value it can
only describe as "an expression" (`tok[:4]`).

### log-key-value-role

**Finds:** a field key that names one quantity while the variable logged under it is another.

```go
slog.Info("retry", "remaining_attempts", elapsedAttempts)
```

Pretty easy to write, hard to spot in review: both are ints, so nothing else complains.

**Reads:** fields whose value is a variable or a field selector. A key whose words match the
variable's (`user_id` and `userID`) is not asked about.

**Sends:** keys and value names as facts. No message.

**Asks:** whether every key names the same quantity as its value, a key conflicts, or the names are
too vague to tell.

**Decides:** reports "conflict". Threshold 0.85. "Too vague" abstains.

### normal-event-at-error

**Finds:** an expected, routine event logged as an error. It fills alerts with noise until people stop
reading them.

```go
log.Error("cache miss, loading from the database")
```

**Reads:** error and fatal level logs with a constant message. Messages that name a failure as a
word ("failed", "cannot", "unable", "invalid") are skipped; "failover" and "failback" are asked about.

**Sends:** the message.

**Asks:** whether the event is routine, a recoverable degradation, or a failure.

**Decides:** reports "routine". Threshold 0.9.

### severe-event-understated

**Finds:** the opposite: lost data or an outage logged at info or debug, where nobody looks.

```go
slog.Info("events for the last hour are lost, the write to disk was refused")
```

**Reads:** debug and info logs with a constant message of at least two words.

**Sends:** the message.

**Asks two questions:** what consequence the message states (routine progress, a temporary
inconvenience, or unintended loss), and whether it describes something done on purpose (a requested
deletion, sampling).

**Decides:** reports when the loss is unintended (threshold 0.85) and "on purpose" is unlikely (0.3
or below). Between 0.3 and 0.7 on the second question it abstains.

### destructive-remediation

**Finds:** a message that tells its reader to delete, wipe, reset, format or reinstall something, and
does not say what will be lost.

```go
return errors.New("state is corrupted, delete the data directory and restart")
```

**Reads:** constant texts of error constructors, and of log calls at warn, error, fatal or panic
level or with no level (`log.Printf`). Debug and info logs are left out to cut the false positives
of a program narrating its own steps ("purge temp files" logged right before the purge). That is a
coverage limit, chosen on purpose: a destructive instruction logged at debug or info is not read.

**Sends:** the text, the kind (error or log message), the log level, and the function called by
the statement after the log call (`Store.Drop`, `os.RemoveAll`), so a warning followed by the
deletion it names reads as the program's own step. This names a call site; it does not prove the
call runs, since an operand that panics or blocks first (`paths[i]`, `*p`, a nil receiver chain,
`<-ch`) is not modelled. The fact is sent only when the next statement is a call, an assignment or
return of one call, or an if whose init is one, and no call in the receiver or arguments comes
first. It is left out after a fatal or panic log and where the code runs the call maybe, later or
never (a branch, a func literal, `defer`, `go`, `&&`).

**Asks two questions:** whether the text gives destructive advice, safe advice, names an action the
program itself is doing or must do, or gives no advice; and whether it says what would be lost or
how to keep it.

**Decides:** reports destructive advice (threshold 0.85) that says nothing about the loss (0.3 or
below on the second question); abstains in between. Safe advice, the program's own action and no
advice are one outcome: a text is clean when their probabilities together reach the threshold, so
a text the classifier splits between "no advice" and "own action" is still clean.

This one asks about every constant message it reads, since a keyword filter missed "format" and
"mkfs". It is the chattiest linter: cheap with Jev, noticeably dearer with an agent backend. Keep a
money cap.

---

## Errors

### sentinel-name-vs-text

**Finds:** a sentinel error whose name says one thing and whose message another, usually a copy-paste.

```go
var ErrQuotaExceeded = errors.New("tenant record not found")
```

**Reads:** package-level variables of type `error` named `Err...`, set from `errors.New` and friends.

**Sends:** the name, its words ("quota exceeded") and the message.

**Asks:** whether they describe the same condition, different conditions, or are too vague.

**Decides:** reports "different" when it is at least 0.55 and ahead of "same" by at least 0.2 (setting
`margin`). If either probability is missing, it abstains.

**Unsupported:** a sentinel that wraps another error, a message built at run time, an empty message.

### error-needs-type (policy, off by default)

**Finds:** a plain string error for a condition a caller will want to branch on: not found, already
exists, permission denied, timeout, closed.

```go
return errors.New("invoice not found") // the caller cannot errors.Is this
```

**Reads:** errors built from a constant message and returned, that do not wrap another error and are
not sentinel declarations.

**Sends:** the message.

**Asks:** whether a caller would plausibly branch on it (yes or no).

**Decides:** reports yes at 0.8 or above.

This is design advice, not a bug, so `default: standard` leaves it off. Enable it by name.

---

## Units and comments

### human-unit-contradiction

**Finds:** a printf message that names one unit while the value is in another.

```go
log.Printf("compaction took %.0f ms", d.Seconds()) // seconds shown as milliseconds
```

**Reads:** printf-style calls (found by signature) where a verb's argument is a `time.Duration`
`Seconds`, `Milliseconds`, `Microseconds`, `Nanoseconds`, `Minutes` or `Hours` call, also through
numeric conversions and `math.Round`. The code knows the unit. Verbs are bound to arguments the way
`fmt` binds them; a test checks the binding against `fmt.Sprintf` itself.

**Sends:** the format and which verb is meant.

**Asks:** which unit the wording gives the number next to that verb, or none.

**Decides:** reports a stated unit that differs from the code's. Threshold 0.85.

**Quiet on:** a Duration printed with `%v` or `%s` (it prints its own unit), a conversion to a type
with its own `String` method.

**Unsupported:** a format built at run time, explicit argument indexes (`%[2]d`).

### enum-comment-shift

**Finds:** a comment on a constant that describes a neighbour, usually after lines were moved or
copied.

```go
const (
	// waiting for a free worker
	PhaseQueued Phase = iota
	// every block has been copied and verified   <- that is PhaseVerified
	PhaseCopying
	PhaseVerified
)
```

Line comments (`PhaseQueued Phase = iota // waiting for a free worker`) are read the same way.

**Reads:** comments of constants in a parenthesised block of at least two.

**Sends:** the comment, with the constant's own name replaced by "this constant" (a classifier trusts
a name over the text around it), and the block's constant names as the options.

**Asks:** which constant the comment describes, or none (a heading or a note).

**Decides:** reports a different constant at 0.8 or above.

**Unsupported:** several constants on one line, blocks of more than 20 constants.

### doc-vs-signature

**Finds:** a doc comment that promises a result the signature does not have, usually after a function
lost its results or its error and the doc was not updated.

```go
// ValidName returns an error if the name is empty.     <- it returns a bool
func ValidName(name string) bool { ... }

// Sync flushes the buffer and returns the number of bytes written.     <- it returns nothing
func (b *Buffer) Sync() { ... }
```

**Reads:** doc comments of functions, methods and interface methods, and their signatures through
`go/types`: whether there are results, and whether one of them is or implements `error`. Two claims
can be checked:

- a function with no results whose doc uses a return word (return, returns, yields, reports
  whether, tells the caller whether, gives back);
- a function with results but no error among them whose doc uses an error word (error, errors, err,
  `ErrX`, `errX`).

A function that already returns an error has nothing to check here. Test, benchmark, fuzz and
example functions in `_test.go` files are skipped (their docs describe other functions; `Testify`
is not one), and so are methods of error types and of interfaces that embed `error` (their docs say
"error" about the receiver). Interfaces are read wherever they are declared, inside a function or
in parentheses too.

**Sends:** only the doc, with the function's own name replaced by "the documented function". No
signature, no types.

**Asks:** one yes/no question: does any sentence of the doc say the function gives back a value
(for a function with no results), or that one of its results is an error (for a function with results
but no error)? These do not count: returning "into a pool", saying when the function returns, a
value it says it sends on a channel, passes to a callback or writes out, an error it logs or passes
on, and a value that carries or formats an error.

**Decides:** reports at 0.85 or above, clean at 0.15 or below, abstains in between (setting
`threshold`).

**Unsupported:** the error claim when a result may hold an error: an interface other than `error`
(`any`, `io.Reader`: its dynamic value may implement `error` too, unless the interface has an `Error`
method of another signature), a type parameter, or an error, such an interface or a type parameter
reachable inside the result (`chan error`, `func() error`, `[]error`, a struct with an error field, at any
depth). The doc may mean that error.

A doc that says only "returns" about something the function sends on a channel or prints
("Describe returns all descriptions" on a method that sends them on `ch`) is reported too. That is a
wording finding rather than a stale contract: the doc is wrong about how the values reach the caller,
and the fix is the verb. A doc that names the delivery ("returns them through ch") is clean.

---

## Tests

### table-case-vs-expectation

**Finds:** a table test case whose name says the opposite of its boolean `want`, usually a copied row
with an unchanged `want`.

```go
{name: "not expired before the deadline", passed: false, want: true},
```

**Reads:** rows bound to a result through the comparison in the loop over the table:
`got := F(tt.in); got != tt.want`, `F(tt.in) != tt.want`, or an `Equal` assertion. A generic `want`
is about the result of that call; a named one (`wantInUse`) is about what its name says. An omitted
field in a keyed row is Go's zero value.

**Sends:** the case name and what the boolean is about ("the result of Expired", "whether in use").

**Asks:** whether the name says the boolean is true, false, or nothing about it.

**Decides:** reports when the name's answer differs from the table. Threshold 0.85. A name that only
describes the input ("passed=false") is clean.

**Unsupported, deliberately strict:** a negated comparison, comparisons with two different calls, a
result variable written again, a function with two bool results; and, in any loop over the table, a
write to the expectation field or the whole row, the row's address taken or the row passed on, or the
table itself reassigned, indexed into or passed on. A write to some other field of the row (an input)
keeps the binding. This binding went through several review rounds; each
loosening found a passing test that got a wrong finding.

### doc-vs-table

**Finds:** a table row that contradicts what the tested function's doc says for that case. Either the
doc or the test is out of date.

```go
// Due reports whether the invoice is past its due date.
...
{name: "three days late", want: false},
```

**Reads:** the same row binding as above, generic `want` only, and the doc of the bound function.

**Sends:** the doc, with the function's name replaced by "this function", and the case name as the
situation.

**Asks:** whether the situation meets the doc's condition for true, does not, or the doc does not say;
and, yes or no, whether the situation describes the case in enough detail to tell without guessing.

**Decides:** reports when met or not met disagrees with `want`. Threshold 0.8. A disagreement in a
case name that only labels its input ("ASCII high" for a tilde) abstains when the classifier is at
least 0.7 sure the name leaves out the facts the condition depends on: its answer about the
condition is then a guess about the input.

**Unsupported:** everything unsupported above, and a function whose doc is in another package
(including the usual external `_test` package).

### test-name-vs-assertion

**Finds:** a test whose name says the call should fail while the test requires success, or the
reverse.

```go
func TestUnquoteRejectsUnbalancedQuote(t *testing.T) {
	_, err := Unquote(`"abc`)
	if err != nil {
		t.Fatal(err) // requires success, though the name promises a rejection
	}
}
```

**Reads:** the function the test is named after (the longest name the test name starts with, ending
on a word boundary), its error, and the check on it: `if err != nil { t.Fatal }`, testify `NoError`,
`Error` and the like.

**Sends:** the test name as words ("unquote rejects unbalanced quote") and the call's name.

**Asks:** whether the name says the call should return an error, succeed, or only describes the input.

**Decides:** reports when the name's answer differs from what the check requires. Threshold 0.9.

**Unsupported, strict again:** two functions match the name; the function is called more than once;
the error is discarded or written again; `ErrorIs` (its target can be nil); anything in the failing
branch that might end the test first (`t.Skip`, `return`, `panic`, a helper call); calls inside the
failure call's arguments other than `err.Error()` on the checked error.

---

## nolint

### suppression-rationale

**Finds:** a `//nolint` reason that explains something other than what the suppressed linter reports.

```go
data, _ := os.ReadFile(p) //nolint:errcheck // the file is small, so reading it whole is fine
```

errcheck reports an unchecked error; the reason is about file size.

**Reads:** `//nolint:<linter> // <reason>` directives naming one linter.

**Sends:** a fixed description of what that linter reports (lintuition's own linters' docs, plus a
built-in table for errcheck, gosec, unused, ineffassign, staticcheck, govet, gocritic, revive, lll,
funlen, gocyclo, dupl, goconst and nestif), and the reason.

**Asks:** whether the reason mentions or addresses what the linter reports, or is about a different
subject. An earlier wording asked whether the reason explains why the report is acceptable here;
a classifier reads almost any reason as a justification, so it called "this function is short"
a fine reason to skip an error check.

**Decides:** reports "a different subject". Threshold 0.8.

**Unsupported:** directives naming several linters, and linters the table does not describe.

---

## Not here yet

Four prototype linters wait for v0.2: `error-message-vs-condition`, `diagnostic-subject-mismatch`,
`diagnostic-polarity-mismatch` and `error-to-http-status`. They need a branch condition, and the
prototype sent it as source. They come back once the condition can be described without its source
text.
