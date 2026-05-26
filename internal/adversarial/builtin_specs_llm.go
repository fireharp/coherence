package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinLLMContradictionSpec() Spec {
	return Spec{
		ID:             "ADV-020-llm-contradiction",
		Description:    "Make a markdown claim contradict a cited markdown source.",
		Operation:      opAddFile,
		TargetKinds:    []graph.NodeKind{graph.NodeDoc},
		ExpectedMeters: []string{"contradiction"},
		RequiresLLM:    true,
		Selector:       Selector{PathGlob: "docs/specs/feature.md"},
		Edit:           Edit{Path: "docs/specs/contradiction.md", Content: "# Contradiction\n\nSee [policy](policy-source.md).\n\nThe policy threshold is 10.\n"},
	}
}
