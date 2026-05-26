package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs146To148() []Spec {
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
		{
			ID:             "ADV-147-json-asset-bare-import-demo",
			Description:    "Remove a JSON asset imported through a bare/root TypeScript specifier; dangling_imports ignores non-relative specs.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "config.json"},
		},
		{
			ID:             "ADV-148-raw-typed-id-trace-coverage-fp-demo",
			Description:    "Replace a valid Markdown story link with raw US-003 text; trace_coverage treats the story as uncovered because mentions edges ignore plain typed IDs.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{},
			Selector:       Selector{PathGlob: "docs/specs/trace.md"},
			Edit: Edit{
				Old: `See [US-003](../user-stories/US-003.md).`,
				New: `See US-003.`,
			},
		},
	}
}
