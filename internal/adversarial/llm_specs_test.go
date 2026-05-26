package adversarial

import (
	"github.com/fireharp/coherence/internal/graph"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLLMSpecs(t *testing.T) {
	specs, err := parseLLMSpecs(`{"version":1,"mutations":[{"id":"LLM-1","operation":"append_text","target_kinds":["file"],"expected_meters":["unknown_id_references"],"selector":{"path_glob":"*.go"},"edit":{"text":"US-999"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "LLM-1" {
		t.Fatalf("specs=%+v", specs)
	}
}

func TestLLMSpecPromptOmitsGraphLabels(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "doc:docs/private.md", Kind: graph.NodeDoc, Label: "Private Roadmap Heading", Path: "docs/private.md"},
			{ID: "claim:abc123", Kind: graph.NodeClaim, Label: "Must never send this claim text", Path: "docs/private.md"},
			{ID: "file:docs/private.md", Kind: graph.NodeFile, Label: "private.md", Path: "docs/private.md"},
		},
		Edges: []graph.Edge{{
			From:       "doc:docs/private.md",
			To:         "claim:abc123",
			Kind:       graph.EdgeDefines,
			Provenance: "secret source sentence",
		}},
	}
	prompt := llmSpecPrompt(g, nil)
	for _, leaked := range []string{"Private Roadmap Heading", "Must never send this claim text", "secret source sentence", "label=", "provenance="} {
		if strings.Contains(prompt, leaked) {
			t.Fatalf("prompt leaked %q:\n%s", leaked, prompt)
		}
	}
	if !strings.Contains(prompt, "path=docs/private.md") ||
		!strings.Contains(prompt, "kind=claim") ||
		!strings.Contains(prompt, "kind=defines from=doc:docs/private.md to=claim:abc123") {
		t.Fatalf("prompt missing graph shape:\n%s", prompt)
	}
}

func TestGenerateLLMSpecsValidatesDryRunAndRecords(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	root := t.TempDir()
	body := `{"choices":[{"message":{"role":"assistant","content":"{\"version\":1,\"mutations\":[{\"id\":\"LLM-good\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"unknown_id_references\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"edit\":{\"text\":\"\\nUS-999\\n\"}},{\"id\":\"LLM-bad\",\"operation\":\"replace_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"broken_links\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"edit\":{\"old\":\"not present in file\",\"new\":\"x\"}}]}"}}]}`
	client := fakeHTTPClient{body: body}
	specs, err := GenerateLLMSpecs(Options{
		RootDir:        root,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: client,
	}, builtinCorpus(), BuiltinSpecs())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "LLM-good" {
		t.Fatalf("specs=%+v, want only dry-run-applicable LLM-good", specs)
	}
	files, err := filepath.Glob(filepath.Join(root, ".coherence", "adversarial", "specs", "llm-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("generated spec files=%v, want one", files)
	}
}

func TestGeneratedLLMSpecsHonorSkipConditions(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	root := t.TempDir()
	body := `{"choices":[{"message":{"role":"assistant","content":"{\"version\":1,\"mutations\":[{\"id\":\"LLM-skipped-file\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"unknown_id_references\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"skip_conditions\":{\"require_files\":[\"missing.txt\"]},\"edit\":{\"text\":\"\\nUS-999\\n\"}},{\"id\":\"LLM-skipped-engine\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"dead_code\"],\"selector\":{\"path_glob\":\"pkg/policy/policy.go\"},\"skip_conditions\":{\"require_optional_engines\":[\"dead_code\"]},\"edit\":{\"text\":\"\\nfunc llmUnused() string { return \\\"x\\\" }\\n\"}}]}"}}]}`
	files := embeddedAgentRepo()
	files["ontology.yml"] = "version: 1\nrules: []\n"
	specs, err := GenerateLLMSpecs(Options{
		RootDir:        root,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: fakeHTTPClient{body: body},
	}, []corpusRepo{{RepoEntry: RepoEntry{ID: "optional-disabled"}, Files: files}}, BuiltinSpecs())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Fatalf("specs=%+v, want generated spec filtered by skip condition", specs)
	}
}

func TestWriteGeneratedSpecsDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	specs := []Spec{{ID: "generated", Operation: opAppendText, TargetKinds: []graph.NodeKind{graph.NodeFile}, ExpectedMeters: []string{"unknown_id_references"}}}
	if err := writeGeneratedSpecs(root, specs); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedSpecs(root, specs); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, ".coherence", "adversarial", "specs", "llm-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("generated spec files=%v, want two unique files", files)
	}
}

func TestRunRecordsLLMSpecExpansionStatus(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: 1, LLMSpecs: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.LLMSpecs.Requested || report.LLMSpecs.Enabled || report.LLMSpecs.Skipped != "missing GROQ_API_KEY" {
		t.Fatalf("unexpected missing-key llm status: %+v", report.LLMSpecs)
	}

	t.Setenv("GROQ_API_KEY", "test-key")
	body := `{"choices":[{"message":{"role":"assistant","content":"not-json"}}]}`
	report, err = Run(Options{
		RootDir:        t.TempDir(),
		Iterations:     1,
		LLMSpecs:       true,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: fakeHTTPClient{body: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.LLMSpecs.Requested || !report.LLMSpecs.Enabled || report.LLMSpecs.Error == "" {
		t.Fatalf("unexpected error llm status: %+v", report.LLMSpecs)
	}
	if !report.Pass {
		t.Fatalf("llm spec expansion error should not fail deterministic run: %+v", report.Summary)
	}
}
