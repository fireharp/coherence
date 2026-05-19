package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func brokenLinksGitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBrokenLinksEmptyRepo(t *testing.T) {
	dir := brokenLinksGitInit(t, nil)
	r := computeBrokenLinks(dir)
	if r.Score != 0 || len(r.Links) != 0 {
		t.Errorf("expected zero broken links, got %+v", r)
	}
}

func TestBrokenLinksTrackedTargetIsNotFlagged(t *testing.T) {
	dir := brokenLinksGitInit(t, map[string]string{
		"docs/index.md":      "See [auth](specs/auth.md).\n",
		"docs/specs/auth.md": "# Auth\n",
	})
	r := computeBrokenLinks(dir)
	if r.Score != 0 {
		t.Errorf("tracked target should not be flagged: %+v", r.Links)
	}
}

func TestBrokenLinksUntrackedTargetIsFlagged(t *testing.T) {
	dir := brokenLinksGitInit(t, map[string]string{
		"docs/index.md":      "See [missing](specs/removed.md) and [extant](specs/auth.md).\n",
		"docs/specs/auth.md": "# Auth\n",
	})
	r := computeBrokenLinks(dir)
	if r.Score != 1 {
		t.Fatalf("expected 1 broken link, got %d: %+v", r.Score, r.Links)
	}
	if r.Links[0].Source != "docs/index.md" || r.Links[0].Target != "docs/specs/removed.md" {
		t.Errorf("unexpected broken link: %+v", r.Links[0])
	}
}

func TestBrokenLinksExternalURLsSkipped(t *testing.T) {
	dir := brokenLinksGitInit(t, map[string]string{
		"docs/index.md": "See [google](https://google.com) and [protocol](mailto:x@y.com).\n",
	})
	r := computeBrokenLinks(dir)
	if r.Score != 0 {
		t.Errorf("external URLs should be skipped, got %+v", r.Links)
	}
}

func TestBrokenLinksDedupsRepeatedTarget(t *testing.T) {
	dir := brokenLinksGitInit(t, map[string]string{
		"docs/index.md": "See [a](specs/removed.md) and [also](specs/removed.md).\n",
	})
	r := computeBrokenLinks(dir)
	if r.Score != 1 {
		t.Errorf("expected 1 deduped broken link, got %d", r.Score)
	}
}

func TestBrokenLinksAbsolutePathResolution(t *testing.T) {
	dir := brokenLinksGitInit(t, map[string]string{
		"docs/index.md": "See [missing](/specs/removed.md).\n",
	})
	r := computeBrokenLinks(dir)
	if r.Score != 1 {
		t.Errorf("expected 1 broken link from absolute path, got %d: %+v", r.Score, r.Links)
	}
	if r.Score == 1 && r.Links[0].Target != "specs/removed.md" {
		t.Errorf("target should resolve to repo-relative: %+v", r.Links[0])
	}
}

func TestBrokenLinksUntrackedButOnDiskNotFlagged(t *testing.T) {
	// Targets that are .gitignored or otherwise untracked but exist on
	// disk (e.g., LOCAL.md notes) shouldn't be flagged — the link works
	// for the user's working tree. Only truly-missing files count.
	dir := brokenLinksGitInit(t, map[string]string{
		"README.md": "See [local notes](LOCAL.md).\n",
	})
	// Create LOCAL.md on disk but do NOT git-add it.
	if err := os.WriteFile(filepath.Join(dir, "LOCAL.md"), []byte("# Local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := computeBrokenLinks(dir)
	if r.Score != 0 {
		t.Errorf("untracked-but-on-disk target should not be flagged, got %+v", r.Links)
	}
}

func TestVerdictTelemetryOnBrokenLinks(t *testing.T) {
	r := Report{BrokenLinks: BrokenLinks{Score: 1, Links: []BrokenLink{{Source: "a.md", Target: "b.md"}}}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}
