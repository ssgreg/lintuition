package cli_test

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormats(t *testing.T) {
	sample(t)
	code, _, errs := run("run",
		"--output.sarif.path", "r.sarif", "--output.checkstyle.path", "r.xml", "--output.code-climate.path", "r.cc.json",
		"--output.junit-xml.path", "r.junit.xml", "--output.github-actions.path", "stdout", "./...")
	if code != 1 {
		t.Fatalf("exit %d %s", code, errs)
	}
	var sarif struct {
		Version string
		Runs    []struct {
			Results []struct {
				RuleID    string
				Level     string
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct{ URI string }
						Region           struct{ StartLine int }
					}
				}
				PartialFingerprints map[string]string
			}
			Invocations []struct{ ExecutionSuccessful bool }
		}
	}
	b, _ := os.ReadFile("r.sarif")
	if err := json.Unmarshal(b, &sarif); err != nil || sarif.Version != "2.1.0" || len(sarif.Runs[0].Results) != 1 {
		t.Fatalf("sarif: %v\n%s", err, b)
	}
	r := sarif.Runs[0].Results[0]
	if r.RuleID != "metric-type-vs-help" || r.Level != "warning" || r.Locations[0].PhysicalLocation.ArtifactLocation.URI != "metrics.go" ||
		r.Locations[0].PhysicalLocation.Region.StartLine != 10 || r.PartialFingerprints["lintuition/v1"] == "" || !sarif.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Fatalf("sarif result: %+v", sarif.Runs[0])
	}
	b, _ = os.ReadFile("r.xml")
	var cs struct {
		Files []struct {
			Name   string `xml:"name,attr"`
			Errors []struct {
				Line   int    `xml:"line,attr"`
				Source string `xml:"source,attr"`
			} `xml:"error"`
		} `xml:"file"`
	}
	if err := xml.Unmarshal(b, &cs); err != nil || cs.Files[0].Name != "metrics.go" || cs.Files[0].Errors[0].Line != 10 {
		t.Fatalf("checkstyle: %v\n%s", err, b)
	}
	b, _ = os.ReadFile("r.cc.json")
	var cc []struct {
		CheckName   string `json:"check_name"`
		Fingerprint string
		Location    struct{ Path string }
	}
	if err := json.Unmarshal(b, &cc); err != nil || len(cc) != 1 || cc[0].Fingerprint == "" || cc[0].Location.Path != "metrics.go" {
		t.Fatalf("code climate: %v\n%s", err, b)
	}
	b, _ = os.ReadFile("r.junit.xml")
	if err := xml.Unmarshal(b, new(struct {
		XMLName xml.Name `xml:"testsuites"`
	})); err != nil || !strings.Contains(string(b), `failures="1"`) {
		t.Fatalf("junit: %v\n%s", err, b)
	}
}

func TestNolintSkipsTheClassifier(t *testing.T) {
	dir := sample(t)
	p := filepath.Join(dir, "metrics.go")
	b, _ := os.ReadFile(p)
	s := strings.Replace(string(b), `Help: ioHelp})`, `Help: ioHelp}) //nolint:metric-type-vs-help // utilization is what the dashboard plots`, 1)
	s = strings.Replace(s, `Help: "Request latency."})`, `Help: "Request latency."}) //nolint:metric-type-vs-help`, 1)
	os.WriteFile(p, []byte(s), 0o600)
	code, out, errs := run("run", "--output.json.path", "stdout", "./...")
	// Counted for metric-type-vs-help alone: suppression-rationale asks about the explained directive.
	if code != 0 || !strings.Contains(out, `"Name":"metric-type-vs-help","Enabled":true,"Candidates":4,"Asked":3,"Abstained":0,"Skipped":1,"SkippedBy":{"nolint":1}`) {
		t.Fatalf("an explained nolint skips the question; an unexplained one does not: exit %d\n%s\n%s", code, out, errs)
	}
}

// TestShowcaseCommand runs the showcase the way the README tells people to, and expects every
// documented example, one finding each, the doubled table row included.
func TestShowcaseCommand(t *testing.T) {
	dir, _ := filepath.Abs("../examples/showcase")
	t.Chdir(dir)
	code, out, errs := run("run", "./...")
	if n := strings.Count(out, "\n") - strings.Count(out, "\n\t"); code != 1 || strings.Count(out, ": ") < 16 {
		t.Fatalf("exit %d, %d lines\n%s\n%s", code, n, out, errs)
	}
	if !strings.Contains(errs, "16 issue(s)") {
		t.Fatalf("the showcase shows 16 findings: %s", errs)
	}
}
