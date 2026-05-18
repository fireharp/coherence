package ids

import (
	"sort"
	"strings"
	"testing"
)

func TestScanWarnsOnlyForMissing(t *testing.T) {
	idx := &Index{
		US:  map[string]struct{}{"US-001": {}},
		ADR: map[string]struct{}{"ADR-020": {}},
		IDR: map[string]struct{}{},
	}
	added := map[string]string{
		"frontend/src/App.tsx": "Refs: US-001 US-999 ADR-020 IDR-001",
	}
	findings := Scan(added, []string{"frontend/src/App.tsx"}, idx)
	rules := []string{}
	for _, f := range findings {
		rules = append(rules, f.Rule)
	}
	sort.Strings(rules)
	want := []string{"unknown-idr-id", "unknown-us-id"}
	if len(rules) != len(want) || rules[0] != want[0] || rules[1] != want[1] {
		t.Errorf("rules = %v, want %v", rules, want)
	}
	// US-999 must appear in the message
	for _, f := range findings {
		if f.Rule == "unknown-us-id" && !strings.Contains(f.Message, "US-999") {
			t.Errorf("missing US-999 in %q", f.Message)
		}
	}
}

func TestScanDedupesPerFile(t *testing.T) {
	idx := &Index{
		US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{},
	}
	added := map[string]string{
		"a.tsx": "US-001 US-001 US-001",
	}
	findings := Scan(added, []string{"a.tsx"}, idx)
	if len(findings) != 1 {
		t.Errorf("got %d findings, want 1", len(findings))
	}
}
