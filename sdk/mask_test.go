package sdk

import "testing"

func TestMask(t *testing.T) {
	for _, tc := range []struct{ text, name, want string }{
		{"Missing reports whether the key is present.", "Missing", "this function reports whether the key is present."},
		{"MissingKey is not Missing.", "Missing", "MissingKey is not this function."},
		{"x is short", "x", "x is short"},
	} {
		if got := Mask(tc.text, tc.name, "this function"); got != tc.want {
			t.Errorf("Mask(%q, %q) = %q, want %q", tc.text, tc.name, got, tc.want)
		}
	}
	got := MaskAll("Get calls GetAll.", map[string]string{"Get": "this function", "GetAll": "the other function"})
	if got != "this function calls the other function." {
		t.Errorf("MaskAll: %q", got)
	}
}
