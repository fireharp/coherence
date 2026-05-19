package exteval

// Samples returns the shipped catalog of external-style evaluation
// samples. One per GOAL.md M7 category, kept tiny on purpose — the goal
// is harness existence + scorability, not benchmark scale. Real SWE-bench
// / TEBench task imports can extend this catalog without touching the
// harness shape.
func Samples() []Sample {
	return []Sample{
		sweAuthBugSample(),
		tebenchPolicySample(),
		docCodeTraceSample(),
	}
}

func sweAuthBugSample() Sample {
	return Sample{
		ID:       "EXT-SWE-001",
		Category: CategorySWEBench,
		Name:     "auth login bug — file localization",
		Description: "Given the auth package source as the seed, predict the test " +
			"and spec doc that should be inspected alongside any fix. Demonstrates " +
			"localization via `verifies` (test→source) and `mentions` (spec→source).",
		Files: map[string]string{
			"go.mod": "module example.com/auth\n",
			"internal/auth/auth.go": `package auth

// Login authenticates a user.
func Login(u, p string) error { return nil }
`,
			"internal/auth/auth_test.go": `package auth

import "testing"

func TestLogin(t *testing.T) {}
`,
			"docs/specs/auth.md": `# Auth Spec

See [the auth package](../../internal/auth/auth.go) for implementation.
`,
		},
		Seed: []string{"internal/auth/auth.go"},
		Gold: []string{
			"internal/auth/auth_test.go",
			"docs/specs/auth.md",
		},
	}
}

func tebenchPolicySample() Sample {
	return Sample{
		ID:       "EXT-TEB-001",
		Category: CategoryTEBench,
		Name:     "policy change — stale test identification",
		Description: "Given a modified policy source as the seed, predict the test " +
			"that needs updating. The Go test path convention wires `verifies` " +
			"automatically; the predictor surfaces the linked test.",
		Files: map[string]string{
			"go.mod": "module example.com/policy\n",
			"pkg/policy.go": `package pkg

func Approve(score int) bool { return score >= 80 }
`,
			"pkg/policy_test.go": `package pkg

import "testing"

func TestApprove(t *testing.T) {}
`,
		},
		Seed: []string{"pkg/policy.go"},
		Gold: []string{"pkg/policy_test.go"},
	}
}

func docCodeTraceSample() Sample {
	return Sample{
		ID:       "EXT-DOC-001",
		Category: CategoryDocCode,
		Name:     "spec ↔ user-story traceability",
		Description: "Given a spec doc as the seed, predict the user-story doc it " +
			"references and the code that implements that story. Exercises " +
			"`mentions` (spec → user-story doc) and `defines` (user-story doc " +
			"→ typed-id node).",
		Files: map[string]string{
			"docs/specs/billing.md": `# Billing Spec

Implements [US-007](../user-stories/US-007.md) for monthly invoicing.
`,
			"docs/user-stories/US-007.md": `---
id: US-007
---
# US-007 — Monthly invoicing
`,
			"docs/user-stories/US-099.md": `---
id: US-099
---
# US-099 — Unrelated story
`,
		},
		Seed: []string{"docs/specs/billing.md"},
		Gold: []string{"docs/user-stories/US-007.md"},
	}
}
