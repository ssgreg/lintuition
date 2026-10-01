package b_test

import (
	"testing"

	"b"
)

func TestIsExpiredExternal(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "external package test", want: true}, // 9 candidate: tested function in the package under test
	}
	for _, tt := range tests {
		_ = b.IsExpired(b.New(tt.want))
	}
}
