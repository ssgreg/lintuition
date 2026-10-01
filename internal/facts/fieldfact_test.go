package facts

import "testing"

func TestFieldFact(t *testing.T) {
	for _, tc := range []struct {
		f    LogField
		want string
		ok   bool
	}{
		{LogField{Key: "password", ValueDesc: "cfg.Password", Type: "string"}, "key password, value cfg.Password, type string", true},
		{LogField{Key: "key", ValueDesc: "k", Type: "[]byte"}, "key key, value k, type slice of byte", true},
		{LogField{Key: "cfg", ValueDesc: "c", Type: "*a.Config"}, "key cfg, value c, type pointer to a.Config", true},
		{LogField{Key: "m", ValueDesc: "m", Type: "map[string]int"}, "key m, value m", true},
		{LogField{Key: "pass:word", ValueDesc: "p", Type: "string"}, "", false},
		{LogField{Key: `x"y`, ValueDesc: "p"}, "", false},
	} {
		got, ok := FieldFact(tc.f)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("FieldFact(%+v) = %q, %v", tc.f, got, ok)
		}
	}
}
