package d_test

import (
	"testing"

	"d"
)

func TestIsExpiredExternal(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "deadline passed", want: true}, // 12 unsupported: the doc is in another package's syntax
	}
	for _, tt := range tests {
		if d.IsExpired(d.New(true)) != tt.want {
			t.Fail()
		}
	}
}
