package cli

import (
	"runtime/debug"
	"testing"
)

func TestDocVersion(t *testing.T) {
	const path = "github.com/ssgreg/lintuition"
	modified := []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}}
	dep := func(v string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: "example.com/custom", Version: "(devel)"},
			Deps: []*debug.Module{{Path: "example.com/plugin", Version: "v1.0.0"}, {Path: path, Version: v, Replace: replace}}}
	}
	for _, c := range []struct {
		name    string
		stamped string
		bi      *debug.BuildInfo
		want    string
	}{
		{"stamped release", "v0.3.0", &debug.BuildInfo{Main: debug.Module{Path: path, Version: "v0.3.0"}}, "v0.3.0"},
		{"stamped, no build info", "v0.3.0", nil, "v0.3.0"},
		{"go install at a tag", "", &debug.BuildInfo{Main: debug.Module{Path: path, Version: "v0.3.0"}}, "v0.3.0"},
		{"stamped, modified tree", "v0.3.0", &debug.BuildInfo{Main: debug.Module{Path: path, Version: "v0.3.0+dirty"}, Settings: modified}, ""},
		{"stamped, vcs.modified only", "v0.3.0", &debug.BuildInfo{Main: debug.Module{Path: path, Version: "v0.3.0"}, Settings: modified}, ""},
		{"stamped, +dirty only", "v0.3.0", &debug.BuildInfo{Main: debug.Module{Path: path, Version: "v0.3.0+dirty"}}, ""},
		{"custom binary", "", dep("v0.3.0", nil), "v0.3.0"},
		{"modified custom main, clean dependency", "", func() *debug.BuildInfo {
			bi := dep("v0.3.0", nil)
			bi.Main.Version, bi.Settings = "v0.0.0-20261005120000-abcdefabcdef+dirty", modified
			return bi
		}(), "v0.3.0"},
		{"stamped, modified custom main", "v0.3.0", func() *debug.BuildInfo {
			bi := dep("v0.3.0", nil)
			bi.Settings = modified
			return bi
		}(), "v0.3.0"},
		{"local directory replace", "", dep("v0.0.0-00010101000000-000000000000", &debug.Module{Path: "../lintuition"}), ""},
		{"fork replace", "", dep("v0.3.0", &debug.Module{Path: "example.com/fork/lintuition", Version: "v0.9.9"}), ""},
		{"same-module replace", "", dep("v0.3.0", &debug.Module{Path: path, Version: "v0.3.1"}), "v0.3.1"},
		{"no lintuition module", "", &debug.BuildInfo{Main: debug.Module{Path: "example.com/other"}}, ""},
	} {
		if got := docVersion(c.stamped, c.bi); got != c.want {
			t.Errorf("%s: docVersion = %q, want %q", c.name, got, c.want)
		}
	}
}
