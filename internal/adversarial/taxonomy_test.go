package adversarial

import (
	"github.com/fireharp/coherence/internal/graph"
	"strings"
	"testing"
)

func TestValidateSpecsRejectsBadOperation(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad",
		Operation:      "explode",
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
	}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuiltinFirstTwentyCoverDeterministicSpecs(t *testing.T) {
	specs := BuiltinSpecs()
	if len(specs) < 21 {
		t.Fatalf("builtin specs=%d, want at least 21", len(specs))
	}
	for i, spec := range specs[:20] {
		if spec.RequiresLLM {
			t.Fatalf("spec %d (%s) requires LLM; first 20 should be deterministic", i, spec.ID)
		}
	}
	if specs[19].ID != "ADV-021-broken-implements-chain" {
		t.Fatalf("20th spec=%s, want ADV-021-broken-implements-chain", specs[19].ID)
	}
	if specs[20].ID != "ADV-020-llm-contradiction" {
		t.Fatalf("21st spec=%s, want ADV-020-llm-contradiction", specs[20].ID)
	}
}

func TestValidateSpecsRejectsUnknownMeterAndEdge(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-meter",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"not_a_meter"},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected unknown meter validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-edge",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		Selector:       Selector{HasIncomingEdge: "not_an_edge"},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected unknown edge validation error")
	}
}

func TestValidateSpecsRejectsBadSkipConditions(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-env",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireEnv: []string{""}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad env skip condition validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-file",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireFiles: []string{"../outside"}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad file skip condition validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-engine",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireOptionalEngines: []string{"not_an_engine"}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad optional engine skip condition validation error")
	}
}

func TestValidateSpecsRejectsUnsafeEditPaths(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-edit-path",
		Operation:      opAddFile,
		TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
		ExpectedMeters: []string{"broken_links"},
		Edit:           Edit{Path: "../outside.md", Content: "x"},
	}})
	if err == nil {
		t.Fatal("expected unsafe edit path validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-rendered-edit-path",
		Operation:      opRenameFile,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		Edit:           Edit{NewPath: "${target_dir}/../../outside.md"},
	}})
	if err == nil {
		t.Fatal("expected unsafe templated edit path validation error")
	}
	for _, p := range []string{".git/config", ".coherence/drift.json"} {
		err = validateSpecs([]Spec{{
			ID:             "bad-reserved-" + strings.ReplaceAll(p, "/", "-"),
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters: []string{"broken_links"},
			Edit:           Edit{Path: p, Content: "x"},
		}})
		if err == nil {
			t.Fatalf("expected reserved edit path validation error for %s", p)
		}
	}
}
