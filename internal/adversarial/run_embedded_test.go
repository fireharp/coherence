package adversarial

import "testing"

func TestRunEmbeddedAdversarialNoLLMKey(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: len(BuiltinSpecs()), Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if report.Pass {
		t.Fatalf("expected exploration demo to keep the run non-passing: %+v", report.Summary)
	}
	if report.Summary.Errored != 0 || report.Summary.FalseNegatives != 126 || report.Summary.FalsePositives != 2 {
		t.Fatalf("unexpected failures: %+v", report.Summary)
	}
	if report.Summary.Skipped != 1 {
		t.Fatalf("skipped=%d, want 1 LLM skip", report.Summary.Skipped)
	}
	assertEmbeddedMisses(t, report.Results)
	assertEmbeddedFalsePositives(t, report.Results)
}
