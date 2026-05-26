package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs146() []Spec {
	return []Spec{
		{
			ID:             "ADV-146-html-anchor-support-path-loss-fp-demo",
			Description:    "Replace a valid Markdown support link with an equivalent HTML anchor; path_loss treats the concept as unsupported because graph mentions only parse Markdown inline links.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{},
			Selector:       Selector{PathGlob: "docs/ref/html-link.md"},
			Edit: Edit{
				Old: `See [US-001](../user-stories/US-001.md).`,
				New: `See <a href="../user-stories/US-001.md">US-001</a>.`,
			},
		},
	}
}
