package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs150() []Spec {
	return []Spec{
		{
			ID:             "ADV-150-url-string-semantic-noop-stale-test-demo",
			Description:    "Change the URL literal in a verified TS source while semantic stripping in stale-tests ignores it due `//` inside the string.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "src/url-policy.ts"},
			Edit: Edit{
				Old: `"https://api/v1"`,
				New: `"https://api/v2"`,
			},
		},
	}
}
