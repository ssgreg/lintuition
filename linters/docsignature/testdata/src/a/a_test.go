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
