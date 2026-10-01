package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// severity maps a configured severity to a format's levels; unset means warning.
func severity(s string, levels map[string]string, def string) string {
	if v, ok := levels[strings.ToLower(s)]; ok {
		return v
	}
	return def
}

// writeSARIF writes SARIF 2.1.0, one result per issue.
func writeSARIF(w io.Writer, issues []Issue, run Run) error {
	type location struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine   int `json:"startLine"`
				StartColumn int `json:"startColumn,omitempty"`
			} `json:"region"`
		} `json:"physicalLocation"`
	}
	type result struct {
		RuleID              string            `json:"ruleId"`
		Level               string            `json:"level"`
		Message             map[string]string `json:"message"`
		Locations           []location        `json:"locations"`
		PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	}
	type rule struct {
		ID string `json:"id"`
	}
	levels := map[string]string{"error": "error", "warning": "warning", "info": "note", "note": "note"}
	var results []result
	ruleSet := map[string]bool{}
	for _, l := range run.Linters {
		if l.Enabled {
			ruleSet[l.Name] = true
		}
	}
	for _, is := range issues {
		var loc location
		loc.PhysicalLocation.ArtifactLocation.URI = strings.ReplaceAll(is.Pos.Filename, "\\", "/")
		loc.PhysicalLocation.Region.StartLine = is.Pos.Line
		loc.PhysicalLocation.Region.StartColumn = is.Pos.Column
		r := result{RuleID: is.FromLinter, Level: severity(is.Severity, levels, "warning"),
			Message: map[string]string{"text": is.Text}, Locations: []location{loc}}
		if is.Fingerprint != "" {
			r.PartialFingerprints = map[string]string{"lintuition/v1": is.Fingerprint}
		}
		results = append(results, r)
		ruleSet[is.FromLinter] = true
	}
	var rules []rule
	for id := range ruleSet {
		rules = append(rules, rule{ID: id})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	if results == nil {
		results = []result{}
	}
	doc := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{
				"name": "lintuition", "informationUri": "https://github.com/ssgreg/lintuition", "rules": rules,
			}},
			"results": results,
			// An incomplete run did not look at everything; say so where SARIF can.
			"invocations": []any{map[string]any{"executionSuccessful": !run.Incomplete}},
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// writeCheckstyle writes Checkstyle XML, issues grouped by file.
func writeCheckstyle(w io.Writer, issues []Issue) error {
	type xmlError struct {
		Line     int    `xml:"line,attr"`
		Column   int    `xml:"column,attr"`
		Severity string `xml:"severity,attr"`
		Message  string `xml:"message,attr"`
		Source   string `xml:"source,attr"`
	}
	type xmlFile struct {
		Name   string     `xml:"name,attr"`
		Errors []xmlError `xml:"error"`
	}
	type doc struct {
		XMLName xml.Name  `xml:"checkstyle"`
		Version string    `xml:"version,attr"`
		Files   []xmlFile `xml:"file"`
	}
	levels := map[string]string{"error": "error", "warning": "warning", "info": "info"}
	d := doc{Version: "5.0"}
	idx := map[string]int{}
	for _, is := range issues {
		i, ok := idx[is.Pos.Filename]
		if !ok {
			i = len(d.Files)
			idx[is.Pos.Filename] = i
			d.Files = append(d.Files, xmlFile{Name: is.Pos.Filename})
		}
		d.Files[i].Errors = append(d.Files[i].Errors, xmlError{
			Line: is.Pos.Line, Column: is.Pos.Column, Severity: severity(is.Severity, levels, "warning"),
			Message: is.Text, Source: is.FromLinter,
		})
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(d); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// writeCodeClimate writes the GitLab Code Quality flavour of Code Climate JSON.
func writeCodeClimate(w io.Writer, issues []Issue) error {
	type item struct {
		Description string `json:"description"`
		CheckName   string `json:"check_name"`
		Severity    string `json:"severity"`
		Fingerprint string `json:"fingerprint"`
		Location    struct {
			Path  string `json:"path"`
			Lines struct {
				Begin int `json:"begin"`
			} `json:"lines"`
		} `json:"location"`
	}
	levels := map[string]string{"blocker": "blocker", "critical": "critical", "error": "major", "major": "major", "warning": "minor", "minor": "minor", "info": "info"}
	out := []item{}
	for _, is := range issues {
		var it item
		it.Description, it.CheckName = is.Text, is.FromLinter
		it.Severity = severity(is.Severity, levels, "minor")
		it.Fingerprint = is.Fingerprint
		it.Location.Path = is.Pos.Filename
		it.Location.Lines.Begin = is.Pos.Line
		out = append(out, it)
	}
	return json.NewEncoder(w).Encode(out)
}

// writeJUnit writes JUnit XML: one test suite per file, one failing test case per issue.
func writeJUnit(w io.Writer, issues []Issue) error {
	type failure struct {
		Message string `xml:"message,attr"`
		Type    string `xml:"type,attr"`
		Body    string `xml:",chardata"`
	}
	type testCase struct {
		Name      string  `xml:"name,attr"`
		ClassName string  `xml:"classname,attr"`
		Failure   failure `xml:"failure"`
	}
	type suite struct {
		Name     string     `xml:"name,attr"`
		Tests    int        `xml:"tests,attr"`
		Failures int        `xml:"failures,attr"`
		Cases    []testCase `xml:"testcase"`
	}
	type doc struct {
		XMLName xml.Name `xml:"testsuites"`
		Suites  []suite  `xml:"testsuite"`
	}
	var d doc
	idx := map[string]int{}
	for _, is := range issues {
		i, ok := idx[is.Pos.Filename]
		if !ok {
			i = len(d.Suites)
			idx[is.Pos.Filename] = i
			d.Suites = append(d.Suites, suite{Name: is.Pos.Filename})
		}
		s := &d.Suites[i]
		s.Tests++
		s.Failures++
		s.Cases = append(s.Cases, testCase{
			Name:      is.FromLinter,
			ClassName: fmt.Sprintf("%s:%d:%d", is.Pos.Filename, is.Pos.Line, is.Pos.Column),
			Failure:   failure{Message: is.Text, Type: is.Severity, Body: strings.Join(is.SourceLines, "\n")},
		})
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(d); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// writeGitHubActions writes workflow commands, ::warning file=...,line=...::message. Data and
// properties are escaped as the runner expects, so a message cannot inject another command.
func writeGitHubActions(w io.Writer, issues []Issue) error {
	levels := map[string]string{"error": "error", "warning": "warning", "info": "notice", "notice": "notice"}
	var b strings.Builder
	for _, is := range issues {
		fmt.Fprintf(&b, "::%s file=%s,line=%d,col=%d,title=%s::%s\n",
			severity(is.Severity, levels, "warning"), ghProp(is.Pos.Filename), is.Pos.Line, is.Pos.Column,
			ghProp(is.FromLinter), ghData(is.Text))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func ghData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func ghProp(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}
