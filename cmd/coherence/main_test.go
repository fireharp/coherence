package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeverityRankMapping(t *testing.T) {
	cases := map[string]int{
		"":      0,
		"info":  0,
		"warn":  1,
		"error": 2,
	}
	for input, want := range cases {
		if got := severityRank(input); got != want {
			t.Errorf("severityRank(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestMergeUniquePreservesFirstOrder(t *testing.T) {
	got := mergeUnique([]string{"a", "b", "c"}, []string{"b", "d", "a"})
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("mergeUnique = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mergeUnique[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeUniqueEmptyInputs(t *testing.T) {
	if got := mergeUnique(nil, nil); len(got) != 0 {
		t.Errorf("mergeUnique(nil, nil) = %v, want empty", got)
	}
	if got := mergeUnique([]string{"a"}, nil); len(got) != 1 || got[0] != "a" {
		t.Errorf("mergeUnique([a], nil) = %v, want [a]", got)
	}
}

func TestResolveOntologyPathDefault(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/ontology.yml" {
		t.Errorf("default = %q, want /repo/ontology.yml", got)
	}
}

func TestResolveOntologyPathAbsolute(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "/etc/coherence/ontology.yml"}}
	got := resolveOntologyPath("/repo", args)
	if got != "/etc/coherence/ontology.yml" {
		t.Errorf("absolute override = %q, want untouched", got)
	}
}

func TestResolveOntologyPathRelative(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "configs/coherence.yml"}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/configs/coherence.yml" {
		t.Errorf("relative override = %q, want /repo/configs/coherence.yml", got)
	}
}

func TestResolveOntologyPathEmptyStringFallsBack(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": ""}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/ontology.yml" {
		t.Errorf("empty override should fall back to default, got %q", got)
	}
}

func TestRunVersionExitsZero(t *testing.T) {
	// Smoke test: both human and JSON modes must return exit 0 even
	// when invoked outside a stamped build (the `(no build info)`
	// fallback path). Catches a future refactor that accidentally
	// errors out on missing VCS info.
	if got := runVersion(false); got != 0 {
		t.Errorf("runVersion(false) = %d, want 0", got)
	}
	if got := runVersion(true); got != 0 {
		t.Errorf("runVersion(true) = %d, want 0", got)
	}
}

func TestStrictPromotionMessageWithRegressions(t *testing.T) {
	got := strictPromotionMessage(3, nil)
	if !strings.Contains(got, "3 regression(s)") {
		t.Errorf("expected regression count in message, got %q", got)
	}
}

func TestStrictPromotionMessageWithoutRegressions(t *testing.T) {
	got := strictPromotionMessage(0, nil)
	if !strings.Contains(got, "drift movement detected") {
		t.Errorf("expected generic movement message, got %q", got)
	}
	if strings.Contains(got, "regression") {
		t.Errorf("zero-count message should not mention regressions, got %q", got)
	}
}

func TestStrictPromotionMessageRealMeterActive(t *testing.T) {
	got := strictPromotionMessage(0, []string{"orphan_endpoints", "neighborhood_drift"})
	if !strings.Contains(got, "real meter(s) active: orphan_endpoints") {
		t.Errorf("expected real-meter call-out, got %q", got)
	}
	if strings.Contains(got, "neighborhood_drift") {
		t.Errorf("movement meter should be filtered out, got %q", got)
	}
}

func TestStrictPromotionMessageOnlyMovementMeters(t *testing.T) {
	got := strictPromotionMessage(0, []string{"neighborhood_drift", "semantic_movement"})
	if !strings.Contains(got, "drift movement detected") {
		t.Errorf("movement-only meters should fall through to generic message, got %q", got)
	}
}

func TestBoolFlagDefaultFalse(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	if boolFlag(args, "missing") {
		t.Error("missing flag should default to false")
	}
}

func TestBoolFlagTrueWhenSet(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"strict": true}}
	if !boolFlag(args, "strict") {
		t.Error("strict flag should be true")
	}
}

func TestStringFlagFallback(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	if got := stringFlag(args, "ontology", "default.yml"); got != "default.yml" {
		t.Errorf("fallback not returned, got %q", got)
	}
}

func TestStringFlagOverride(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "custom.yml"}}
	if got := stringFlag(args, "ontology", "default.yml"); got != "custom.yml" {
		t.Errorf("override not returned, got %q", got)
	}
}

func TestIntFlagFallbackAndOverride(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"iterations": "7", "bad": "nope"}}
	if got := intFlag(args, "iterations", 3); got != 7 {
		t.Errorf("iterations=%d, want 7", got)
	}
	if got := intFlag(args, "bad", 3); got != 3 {
		t.Errorf("bad fallback=%d, want 3", got)
	}
	if got := intFlag(args, "missing", 5); got != 5 {
		t.Errorf("missing fallback=%d, want 5", got)
	}
}

func TestParseArgsHandlesEqualsAndBare(t *testing.T) {
	args := parseArgs([]string{"--json", "--ontology=custom.yml", "--strict"})
	if !boolFlag(args, "json") || !boolFlag(args, "strict") {
		t.Errorf("bare flags lost: %+v", args.flags)
	}
	if got := stringFlag(args, "ontology", ""); got != "custom.yml" {
		t.Errorf("--ontology=... lost: %q", got)
	}
}

func TestUsageMentionsAdversarialFlags(t *testing.T) {
	for _, flag := range []string{
		"--corpus-manifest",
		"--iterations",
		"--seed",
		"--jobs",
		"--taxonomy",
		"--llm-specs",
		"--refine-from",
		"--cycles",
		"--export-report",
	} {
		if !strings.Contains(usage, flag) {
			t.Fatalf("usage missing adversarial flag %s", flag)
		}
	}
}

func TestUsageMentionsLifecycleSuite(t *testing.T) {
	if !strings.Contains(usage, "lifecycle") {
		t.Fatal("usage missing lifecycle suite")
	}
}

func TestRunBenchLifecycleSmoke(t *testing.T) {
	args := parseArgs([]string{"--suite=lifecycle", "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, t.TempDir()) })
	if exit != 0 {
		t.Fatalf("runBench lifecycle = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		Pass   bool `json:"pass"`
		Counts struct {
			Steps int `json:"steps"`
			Fail  int `json:"fail"`
		} `json:"counts"`
		FinalHealth map[string]int `json:"final_health"`
		Results     []struct {
			Lane         string   `json:"lane"`
			StepID       string   `json:"step_id"`
			ActiveMeters []string `json:"active_meters"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if !payload.Pass || payload.Counts.Steps != 6 || payload.Counts.Fail != 0 {
		t.Fatalf("unexpected lifecycle payload: %+v", payload)
	}
	if payload.FinalHealth["managed"] != 100 || payload.FinalHealth["unmanaged"] != 0 {
		t.Fatalf("final health=%v, want managed=100 unmanaged=0", payload.FinalHealth)
	}
	if len(payload.Results) != 12 {
		t.Fatalf("results=%d, want 12", len(payload.Results))
	}
	last := payload.Results[len(payload.Results)-1]
	if last.Lane != "unmanaged" || last.StepID != "generated-artifact" || !containsString(last.ActiveMeters, "required_edge_breakage") {
		t.Fatalf("unexpected final result: %+v", last)
	}
}

func TestRunBenchLifecycleWriteReportSmoke(t *testing.T) {
	root := t.TempDir()
	args := parseArgs([]string{"--suite=lifecycle", "--write-report", "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench lifecycle write-report = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		ReportPaths map[string]string `json:"report_paths"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	for _, key := range []string{"json", "html"} {
		if payload.ReportPaths[key] == "" {
			t.Fatalf("missing report path %s in output:\n%s", key, out)
		}
		if _, err := os.Stat(payload.ReportPaths[key]); err != nil {
			t.Fatalf("missing lifecycle report %s: %v", key, err)
		}
	}
}

func TestRunBenchAdversarialSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--json"})
	if got := runBench(args, t.TempDir()); got != 0 {
		t.Fatalf("runBench adversarial = %d, want 0", got)
	}
}

func TestRunBenchAdversarialCyclesSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--cycles=2", "--json"})
	if got := runBench(args, t.TempDir()); got != 0 {
		t.Fatalf("runBench adversarial cycles = %d, want 0", got)
	}
}

func TestRunBenchAdversarialJobsSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	args := parseArgs([]string{"--suite=adversarial", "--iterations=8", "--jobs=4", "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, t.TempDir()) })
	if exit != 0 {
		t.Fatalf("runBench adversarial jobs = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		Iterations int `json:"iterations"`
		Summary    struct {
			Errored int `json:"errored"`
		} `json:"summary"`
		Results []struct {
			Iteration int `json:"iteration"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if payload.Iterations != 8 || len(payload.Results) != 8 || payload.Summary.Errored != 0 {
		t.Fatalf("unexpected jobs payload: %+v", payload)
	}
	for i, result := range payload.Results {
		if result.Iteration != i+1 {
			t.Fatalf("results not sorted by iteration at %d: %+v", i, payload.Results)
		}
	}
}

func TestRunBenchAdversarialRejectsInvalidNumericFlags(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	cases := [][]string{
		{"--iterations=0"},
		{"--iterations=-1"},
		{"--iterations=1x"},
		{"--iterations"},
		{"--jobs=0"},
		{"--cycles=0"},
		{"--seed=nope"},
	}
	for _, extra := range cases {
		args := parseArgs(append([]string{"--suite=adversarial", "--json"}, extra...))
		if got := runBench(args, t.TempDir()); got != 2 {
			t.Fatalf("runBench(%v) = %d, want 2", extra, got)
		}
	}
}

func TestRunBenchAdversarialExportReportSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--export-report=docs/adversarial.md", "--json"})
	if got := runBench(args, root); got != 0 {
		t.Fatalf("runBench adversarial export = %d, want 0", got)
	}
	body, err := os.ReadFile(filepath.Join(root, "docs", "adversarial.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Adversarial Coherence Bench") || !strings.Contains(string(body), "Mutation Results") {
		t.Fatalf("exported report missing title:\n%s", body)
	}
}

func TestRunBenchAdversarialManifestInputSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	repo := createTinyGitRepo(t, filepath.Join(root, "fixture"))
	manifest := filepath.Join(root, "corpus.yml")
	if err := os.WriteFile(manifest, []byte("version: 1\nrepos:\n  - id: local-fixture\n    path: fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--corpus-manifest=" + manifest, "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench manifest = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		Repos   []string `json:"repos"`
		Results []struct {
			RepoID string `json:"repo_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if len(payload.Repos) != 1 || payload.Repos[0] != "local-fixture" {
		t.Fatalf("repos=%v, want [local-fixture]", payload.Repos)
	}
	if len(payload.Results) != 1 || payload.Results[0].RepoID != "local-fixture" {
		t.Fatalf("results=%+v, want repo_id local-fixture", payload.Results)
	}
	if status := gitOutput(t, repo, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Fatalf("fixture repo dirtied by manifest run:\n%s", status)
	}
}

func TestRunBenchAdversarialRelativeInputsResolveFromRoot(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	t.Chdir(t.TempDir())
	repo := createTinyGitRepo(t, filepath.Join(root, "fixture"))
	if err := os.WriteFile(filepath.Join(root, "corpus.yml"), []byte("version: 1\nrepos:\n  - id: local-fixture\n    path: fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "taxonomy.yml"), []byte(`version: 1
mutations:
  - id: cli-taxonomy
    operation: append_text
    target_kinds: [file]
    expected_meters: [unknown_id_references]
    selector:
      path_glob: AGENTS.md
    edit:
      text: "\nUS-999\n"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--corpus-manifest=corpus.yml", "--taxonomy=taxonomy.yml", "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench relative inputs = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		Repos []string `json:"repos"`
		Specs []string `json:"specs"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if len(payload.Repos) != 1 || payload.Repos[0] != "local-fixture" {
		t.Fatalf("repos=%v, want [local-fixture]", payload.Repos)
	}
	if !containsString(payload.Specs, "cli-taxonomy") {
		t.Fatalf("specs=%v, want cli-taxonomy", payload.Specs)
	}
	if status := gitOutput(t, repo, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Fatalf("fixture repo dirtied by relative input run:\n%s", status)
	}
}

func TestRunBenchAdversarialWriteReportSmoke(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	args := parseArgs([]string{"--suite=adversarial", "--iterations=1", "--write-report", "--json"})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench write-report = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		ReportDir string `json:"report_dir"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if payload.ReportDir == "" {
		t.Fatalf("missing report_dir in output:\n%s", out)
	}
	for _, rel := range []string{"iterations.jsonl", "summary.json", "clusters.md", "refinements.json"} {
		if _, err := os.Stat(filepath.Join(payload.ReportDir, rel)); err != nil {
			t.Fatalf("missing report artifact %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".coherence", "adversarial", "leaderboard.json")); err != nil {
		t.Fatalf("missing leaderboard: %v", err)
	}
}

func TestRunBenchAdversarialStrictMissFails(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	taxonomyPath, runDir := writeAdversarialMissInputs(t, root)
	args := parseArgs([]string{
		"--suite=adversarial",
		"--iterations=1",
		"--strict",
		"--json",
		"--taxonomy=" + taxonomyPath,
		"--refine-from=" + runDir,
	})
	if got := runBench(args, root); got != 1 {
		t.Fatalf("runBench adversarial strict miss = %d, want 1", got)
	}
}

func TestRunBenchAdversarialRelativeRefineFromResolvesFromRoot(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	t.Chdir(t.TempDir())
	taxonomyPath, runDir := writeAdversarialMissInputs(t, root)
	relRunDir, err := filepath.Rel(root, runDir)
	if err != nil {
		t.Fatal(err)
	}
	args := parseArgs([]string{
		"--suite=adversarial",
		"--iterations=1",
		"--json",
		"--taxonomy=" + taxonomyPath,
		"--refine-from=" + relRunDir,
	})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench relative refine-from = %d, want 0\n%s", exit, out)
	}
	var payload struct {
		RefineFrom string `json:"refine_from"`
		Seed       int    `json:"seed"`
		Results    []struct {
			MutationID string `json:"mutation_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if payload.RefineFrom != relRunDir || payload.Seed != 2 {
		t.Fatalf("payload refine/seed=(%q,%d), want (%q,2)", payload.RefineFrom, payload.Seed, relRunDir)
	}
	if len(payload.Results) != 1 || payload.Results[0].MutationID != "ZZ-FN" {
		t.Fatalf("results=%+v, want refined mutation ZZ-FN", payload.Results)
	}
}

func TestRunBenchAllAdversarialTelemetryDoesNotFailNonStrict(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	taxonomyPath, runDir := writeAdversarialMissInputs(t, root)
	args := parseArgs([]string{
		"--suite=all",
		"--iterations=1",
		"--json",
		"--taxonomy=" + taxonomyPath,
		"--refine-from=" + runDir,
	})
	exit, out := captureStdout(t, func() int { return runBench(args, root) })
	if exit != 0 {
		t.Fatalf("runBench all non-strict adversarial telemetry = %d, want 0", exit)
	}
	var payload struct {
		Pass        bool `json:"pass"`
		Adversarial struct {
			Pass bool `json:"pass"`
		} `json:"adversarial"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if !payload.Pass {
		t.Fatalf("combined pass=false in non-strict telemetry mode:\n%s", out)
	}
	if payload.Adversarial.Pass {
		t.Fatalf("test setup expected nested adversarial miss:\n%s", out)
	}
}

func writeAdversarialMissInputs(t *testing.T, root string) (string, string) {
	t.Helper()
	taxonomyPath := filepath.Join(root, "taxonomy.yml")
	if err := os.WriteFile(taxonomyPath, []byte(`version: 1
mutations:
  - id: ZZ-FN
    operation: append_text
    target_kinds: [file]
    expected_meters: [broken_links]
    selector:
      path_glob: AGENTS.md
    edit:
      text: "\nplain text with no broken link\n"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".coherence", "adversarial", "runs", "prior")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.json"), []byte(`{
  "run_id": "prior",
  "generated_at": "2026-05-25T00:00:00Z",
  "seed": 1,
  "iterations": 1,
  "clusters": [{"key": "k", "count": 1, "mutation_ids": ["ZZ-FN"]}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return taxonomyPath, runDir
}

func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	exit := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return exit, string(data)
}

func createTinyGitRepo(t *testing.T, dir string) string {
	t.Helper()
	files := map[string]string{
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
		"README.md":    "# Fixture\n",
	}
	for rel, body := range files {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitOutput(t, dir, "init", "-q")
	gitOutput(t, dir, "add", "-A")
	gitOutput(t, dir,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	)
	return dir
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func containsString(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}
