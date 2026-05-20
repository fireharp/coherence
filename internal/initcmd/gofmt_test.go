package initcmd

import (
	"os/exec"
	"strings"
	"testing"
)

// TestGofmtCleanAcrossRepo runs `gofmt -l ./...` from the repo root
// and fails when any tracked .go file would be reformatted. Modern
// gofmt (Go 1.19+) tightened doc-comment list formatting, and the
// codebase drifted between iter 1 and iter 146 — this test prevents
// that from happening again. If it fails, run `gofmt -w internal/
// cmd/` to fix.
//
// Lives in initcmd rather than a top-level package because Go test
// packages can't easily address "the whole module" without a build
// tag. initcmd already exec's external commands (`git config`) so
// the exec-from-test pattern fits.
func TestGofmtCleanAcrossRepo(t *testing.T) {
	// Resolve the repo root by walking up from the test file's
	// working dir (which is the package dir) until we find go.mod.
	repoRoot, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Skipf("go env GOMOD failed: %v", err)
	}
	mod := strings.TrimSpace(string(repoRoot))
	if mod == "" || mod == "/dev/null" {
		t.Skip("not in a Go module")
	}
	root := strings.TrimSuffix(mod, "/go.mod")

	cmd := exec.Command("gofmt", "-l", "cmd", "internal")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gofmt -l failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("gofmt drift detected. Run `gofmt -w cmd/ internal/` to fix:\n%s", out)
	}
}
