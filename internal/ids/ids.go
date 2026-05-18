// Package ids builds an index of US-/ADR-/IDR- IDs defined under
// docs/user-stories and docs/decisions, and scans staged additions for
// references to undefined IDs.
package ids

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"coherence/internal/git"
)

type Index struct {
	US  map[string]struct{}
	ADR map[string]struct{}
	IDR map[string]struct{}
}

func newIndex() *Index {
	return &Index{
		US:  map[string]struct{}{},
		ADR: map[string]struct{}{},
		IDR: map[string]struct{}{},
	}
}

func (i *Index) has(label, id string) bool {
	var m map[string]struct{}
	switch label {
	case "US":
		m = i.US
	case "ADR":
		m = i.ADR
	case "IDR":
		m = i.IDR
	default:
		return false
	}
	_, ok := m[id]
	return ok
}

func (i *Index) add(label, id string) {
	switch label {
	case "US":
		i.US[id] = struct{}{}
	case "ADR":
		i.ADR[id] = struct{}{}
	case "IDR":
		i.IDR[id] = struct{}{}
	}
}

var (
	idPatterns = []struct {
		Label string
		Re    *regexp.Regexp
	}{
		{"US", regexp.MustCompile(`\bUS-\d{3}\b`)},
		{"ADR", regexp.MustCompile(`\bADR-\d{3}\b`)},
		{"IDR", regexp.MustCompile(`\bIDR-\d{3}\b`)},
	}
	frontmatterRe = map[string]*regexp.Regexp{
		"ADR": regexp.MustCompile(`(?m)^id:\s*(ADR-\d{3})\s*$`),
		"IDR": regexp.MustCompile(`(?m)^id:\s*(IDR-\d{3})\s*$`),
	}
)

// Build returns an Index of defined IDs. Mirrors lib/ids.mjs:buildIdIndex.
func Build(rootDir string) *Index {
	idx := newIndex()
	tracked := git.LsFiles(rootDir, "docs/decisions", "docs/user-stories")
	staged := git.StagedNameOnlyIn(rootDir, "docs/decisions", "docs/user-stories")

	seen := map[string]bool{}
	files := []string{}
	for _, l := range append(tracked, staged...) {
		if seen[l] {
			continue
		}
		seen[l] = true
		files = append(files, l)
	}

	for _, rel := range files {
		base := filepath.Base(rel)
		if strings.HasPrefix(rel, "docs/user-stories/") {
			if m := idPatterns[0].Re.FindString(base); m != "" {
				idx.add("US", m)
			}
		}
		if strings.HasPrefix(rel, "docs/decisions/") {
			for _, label := range []string{"ADR", "IDR"} {
				labelRe := regexp.MustCompile(`\b` + label + `-\d{3}\b`)
				if m := labelRe.FindString(base); m != "" {
					idx.add(label, m)
				}
				abs := filepath.Join(rootDir, rel)
				text, err := os.ReadFile(abs)
				if err != nil {
					continue
				}
				if fm := frontmatterRe[label].FindStringSubmatch(string(text)); fm != nil {
					idx.add(label, fm[1])
				}
			}
		}
	}
	return idx
}

// UnknownFinding describes an ID referenced in a non-Markdown staged addition
// that has no matching defined record.
type UnknownFinding struct {
	Rule          string   `json:"rule"`
	Severity      string   `json:"severity"`
	Message       string   `json:"message"`
	TriggeredBy   []string `json:"triggered_by"`
	ExpectedAnyOf []string `json:"expected_any_of"`
}

// Scan walks the per-file added-content map and returns findings for IDs that
// don't appear in the index. Order: per-file (input order), per-pattern (US,
// ADR, IDR), per-match (first appearance only — duplicates within a single
// file are suppressed).
func Scan(addedByPath map[string]string, fileOrder []string, idx *Index) []UnknownFinding {
	findings := []UnknownFinding{}
	for _, filePath := range fileOrder {
		text, ok := addedByPath[filePath]
		if !ok || text == "" {
			continue
		}
		for _, pat := range idPatterns {
			seen := map[string]bool{}
			for _, m := range pat.Re.FindAllString(text, -1) {
				if seen[m] {
					continue
				}
				seen[m] = true
				if !idx.has(pat.Label, m) {
					dir := "decisions"
					if pat.Label == "US" {
						dir = "user-stories"
					}
					findings = append(findings, UnknownFinding{
						Rule:          fmt.Sprintf("unknown-%s-id", strings.ToLower(pat.Label)),
						Severity:      "warn",
						Message:       fmt.Sprintf("%s mentioned in %s but no matching %s record exists", m, filePath, pat.Label),
						TriggeredBy:   []string{filePath},
						ExpectedAnyOf: []string{fmt.Sprintf("docs/%s/**/%s*.md", dir, m)},
					})
				}
			}
		}
	}
	return findings
}
