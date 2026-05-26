package adversarial

import (
	"github.com/fireharp/coherence/internal/graph"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFirstTwentyHitsEveryDeterministicMeter(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: 20, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pass || report.Summary.Skipped != 0 || report.Summary.Errored != 0 {
		t.Fatalf("expected clean deterministic run: %+v", report.Summary)
	}
	expectedMeters := map[string]bool{}
	for _, spec := range BuiltinSpecs()[:20] {
		if spec.RequiresLLM {
			t.Fatalf("deterministic window includes llm spec %s", spec.ID)
		}
		for _, meter := range spec.ExpectedMeters {
			expectedMeters[meter] = true
		}
	}
	for meter := range expectedMeters {
		stats, ok := report.Summary.ByExpectedMeter[meter]
		if !ok {
			t.Fatalf("missing expected meter %s in summary", meter)
		}
		if stats.Total == 0 || stats.Hits != stats.Total || stats.FalseNegatives != 0 || stats.FalsePositives != 0 {
			t.Fatalf("meter %s stats=%+v, want all hits", meter, stats)
		}
	}
	var noop *Result
	for i := range report.Results {
		if report.Results[i].MutationID == "ADV-014-semantic-noop-typo" {
			noop = &report.Results[i]
			break
		}
	}
	if noop == nil {
		t.Fatal("missing semantic noop result")
	}
	if noop.Classification != ClassificationHit || len(noop.ActualMeters) != 0 {
		t.Fatalf("semantic noop result=%+v, want hit with no active meters", *noop)
	}
}

func TestRunSkipsOnExplicitConditions(t *testing.T) {
	t.Setenv("ADV_TEST_REQUIRED_ENV", "")
	envSpec := Spec{
		ID:             "skip-env",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"unknown_id_references"},
		Selector:       Selector{PathGlob: "AGENTS.md"},
		SkipConditions: SkipConditions{RequireEnv: []string{"ADV_TEST_REQUIRED_ENV"}},
		Edit:           Edit{Text: "\nUS-999\n"},
	}
	res := runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: envSpec, seed: 1})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "ADV_TEST_REQUIRED_ENV") {
		t.Fatalf("env skip result=%+v", res)
	}
	fileSpec := envSpec
	fileSpec.ID = "skip-file"
	fileSpec.SkipConditions = SkipConditions{RequireFiles: []string{"missing.txt"}}
	res = runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: fileSpec, seed: 1})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "missing.txt") {
		t.Fatalf("file skip result=%+v", res)
	}

	files := embeddedAgentRepo()
	files["ontology.yml"] = "version: 1\nrules: []\n"
	engineSpec := BuiltinSpecs()[17]
	res = runOne("run", workItem{
		iteration: 1,
		repo:      corpusRepo{RepoEntry: RepoEntry{ID: "optional-disabled"}, Files: files},
		spec:      engineSpec,
		seed:      1,
	})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "callsite_blast_radius") {
		t.Fatalf("optional-engine skip result=%+v", res)
	}
}

func TestRunParallelMatchesSerial(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	serial, err := Run(Options{RootDir: root, Iterations: 8, Seed: 9, Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	parallel, err := Run(Options{RootDir: root, Iterations: 8, Seed: 9, Jobs: 4})
	if err != nil {
		t.Fatal(err)
	}
	if serial.Summary.Hits != parallel.Summary.Hits ||
		serial.Summary.FalseNegatives != parallel.Summary.FalseNegatives ||
		serial.Summary.FalsePositives != parallel.Summary.FalsePositives ||
		serial.Summary.Skipped != parallel.Summary.Skipped ||
		serial.Summary.Errored != parallel.Summary.Errored {
		t.Fatalf("summary mismatch:\nserial=%+v\nparallel=%+v", serial.Summary, parallel.Summary)
	}
	serialSig := resultSignatures(serial.Results)
	parallelSig := resultSignatures(parallel.Results)
	if strings.Join(serialSig, "\n") != strings.Join(parallelSig, "\n") {
		t.Fatalf("result signatures differ:\nserial=%v\nparallel=%v", serialSig, parallelSig)
	}
}

func TestRunManifestRepoDoesNotMutateSource(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, embeddedAgentRepo()); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(source, "pkg", "policy", "policy.go")
	before, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "corpus.yml")
	if err := os.WriteFile(manifestPath, []byte("version: 1\nrepos:\n  - id: local\n    path: source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Run(Options{RootDir: root, ManifestPath: manifestPath, Iterations: 1, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].MutationID != "ADV-001-stale-go-test" {
		t.Fatalf("unexpected result: %+v", report.Results)
	}
	after, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("source repo file changed during adversarial run")
	}
	status, err := runGit(source, "status", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("source repo dirty after adversarial run:\n%s", status)
	}
}
