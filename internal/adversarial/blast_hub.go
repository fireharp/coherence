package adversarial

import "strings"

func blastHubContent() string {
	links := []string{
		"[feature](specs/feature.md)",
		"[policy](specs/policy-source.md)",
		"[story](user-stories/US-001.md)",
		"[decision](decisions/ADR-001.md)",
		"[target](ref/target.md)",
		"[api](../src/api.ts)",
		"[api test](../src/api.test.ts)",
		"[policy source](../pkg/policy/policy.go)",
		"[policy test](../pkg/policy/policy_test.go)",
		"[metric](../metrics/signup_rate.yaml)",
		"[fixture builder](../src/build-fixtures.go)",
		"[fixture](../fixtures/dashboard.json)",
	}
	return "# Adversarial Hub\n\n" + strings.Join(links, "\n") + "\n"
}
