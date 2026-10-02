package a

import "testing"

// TestValidate checks that Validate returns an error on an empty name.
func TestValidate(t *testing.T) {} // 30 none: a test describes another function's result

// ExampleRun returns when ctx is done.
func ExampleRun() {} // 31 none: an example

// BenchmarkSync returns the bytes per op.
func BenchmarkSync(b *testing.B) {} // 32 none: a benchmark

// helper returns the fixture path.
func helper() {} // 33 result: a helper in a test file is an ordinary function

// Testify returns the number of completed operations.
func Testify() {} // 60 result: Testify is not a test name

// Test_lower returns nothing useful.
func Test_lower(t *testing.T) {} // 61 none: Test_ is a test name

// Test returns the bare name.
func Test(t *testing.T) {} // 62 none: the bare prefix is a test name

// Examplefoo returns a value.
func Examplefoo() {} // 63 result: Examplefoo is not an example name
