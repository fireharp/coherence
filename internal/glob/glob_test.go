package glob

import "testing"

func TestMatchSingleStarAndRecursive(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"rill/metrics/*.yaml", "rill/metrics/model_costs.yaml", true},
		{"rill/metrics/*.yaml", "rill/metrics/nested/model_costs.yaml", false},
		{"docs/**/*.md", "docs/coverage.md", true},
		{"docs/**/*.md", "docs/user-stories/index.md", true},
		{"docs/user-stories/**/US-*.md", "docs/user-stories/epics/executive-overview/US-001-org-health-summary.md", true},
		{"frontend/scripts/build-fixtures.mjs", "frontend/scripts/build-fixtures.mjs", true},
		{"frontend/scripts/build-fixtures.mjs", "frontend/scripts/build-fixtures.ts", false},
		{"a/**", "a/b/c.txt", true},
		{"a/**", "a/", true},
		{"*.json", "config.json", true},
		{"*.json", "sub/config.json", false},
		{"?ello.txt", "hello.txt", true},
		{"?ello.txt", "h/llo.txt", false},
	}
	for _, c := range cases {
		got := Match(c.pattern, c.path)
		if got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestAnyMatches(t *testing.T) {
	if !AnyMatches([]string{"docs/**/*.md", "rill/*.yaml"}, []string{"rill/x.yaml"}) {
		t.Error("expected match")
	}
	if AnyMatches([]string{"docs/**/*.md"}, []string{"src/main.go"}) {
		t.Error("unexpected match")
	}
}

func TestTriggeredGlobs(t *testing.T) {
	globs := []string{"a/*.go", "b/*.go"}
	paths := []string{"a/x.go", "c/y.go"}
	got := TriggeredGlobs(globs, paths)
	if len(got) != 1 || got[0] != "a/*.go" {
		t.Errorf("got %v", got)
	}
}
