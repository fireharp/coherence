package main

import (
	"strings"
	"testing"
)

func TestSeverityRankMapping(t *testing.T) {
	cases := map[string]int{
		"":      0,
		"info":  0,
		"warn":  1,
		"error": 2,
	}
	for input, want := range cases {
		if got := severityRank(input); got != want {
			t.Errorf("severityRank(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestMergeUniquePreservesFirstOrder(t *testing.T) {
	got := mergeUnique([]string{"a", "b", "c"}, []string{"b", "d", "a"})
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("mergeUnique = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mergeUnique[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeUniqueEmptyInputs(t *testing.T) {
	if got := mergeUnique(nil, nil); len(got) != 0 {
		t.Errorf("mergeUnique(nil, nil) = %v, want empty", got)
	}
	if got := mergeUnique([]string{"a"}, nil); len(got) != 1 || got[0] != "a" {
		t.Errorf("mergeUnique([a], nil) = %v, want [a]", got)
	}
}

func TestResolveOntologyPathDefault(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/ontology.yml" {
		t.Errorf("default = %q, want /repo/ontology.yml", got)
	}
}

func TestResolveOntologyPathAbsolute(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "/etc/coherence/ontology.yml"}}
	got := resolveOntologyPath("/repo", args)
	if got != "/etc/coherence/ontology.yml" {
		t.Errorf("absolute override = %q, want untouched", got)
	}
}

func TestResolveOntologyPathRelative(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "configs/coherence.yml"}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/configs/coherence.yml" {
		t.Errorf("relative override = %q, want /repo/configs/coherence.yml", got)
	}
}

func TestResolveOntologyPathEmptyStringFallsBack(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": ""}}
	got := resolveOntologyPath("/repo", args)
	if got != "/repo/ontology.yml" {
		t.Errorf("empty override should fall back to default, got %q", got)
	}
}

func TestRunVersionExitsZero(t *testing.T) {
	// Smoke test: both human and JSON modes must return exit 0 even
	// when invoked outside a stamped build (the `(no build info)`
	// fallback path). Catches a future refactor that accidentally
	// errors out on missing VCS info.
	if got := runVersion(false); got != 0 {
		t.Errorf("runVersion(false) = %d, want 0", got)
	}
	if got := runVersion(true); got != 0 {
		t.Errorf("runVersion(true) = %d, want 0", got)
	}
}

func TestStrictPromotionMessageWithRegressions(t *testing.T) {
	got := strictPromotionMessage(3, nil)
	if !strings.Contains(got, "3 regression(s)") {
		t.Errorf("expected regression count in message, got %q", got)
	}
}

func TestStrictPromotionMessageWithoutRegressions(t *testing.T) {
	got := strictPromotionMessage(0, nil)
	if !strings.Contains(got, "drift movement detected") {
		t.Errorf("expected generic movement message, got %q", got)
	}
	if strings.Contains(got, "regression") {
		t.Errorf("zero-count message should not mention regressions, got %q", got)
	}
}

func TestStrictPromotionMessageRealMeterActive(t *testing.T) {
	got := strictPromotionMessage(0, []string{"orphan_endpoints", "neighborhood_drift"})
	if !strings.Contains(got, "real meter(s) active: orphan_endpoints") {
		t.Errorf("expected real-meter call-out, got %q", got)
	}
	if strings.Contains(got, "neighborhood_drift") {
		t.Errorf("movement meter should be filtered out, got %q", got)
	}
}

func TestStrictPromotionMessageOnlyMovementMeters(t *testing.T) {
	got := strictPromotionMessage(0, []string{"neighborhood_drift", "semantic_movement"})
	if !strings.Contains(got, "drift movement detected") {
		t.Errorf("movement-only meters should fall through to generic message, got %q", got)
	}
}

func TestBoolFlagDefaultFalse(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	if boolFlag(args, "missing") {
		t.Error("missing flag should default to false")
	}
}

func TestBoolFlagTrueWhenSet(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"strict": true}}
	if !boolFlag(args, "strict") {
		t.Error("strict flag should be true")
	}
}

func TestStringFlagFallback(t *testing.T) {
	args := parsedArgs{flags: map[string]any{}}
	if got := stringFlag(args, "ontology", "default.yml"); got != "default.yml" {
		t.Errorf("fallback not returned, got %q", got)
	}
}

func TestStringFlagOverride(t *testing.T) {
	args := parsedArgs{flags: map[string]any{"ontology": "custom.yml"}}
	if got := stringFlag(args, "ontology", "default.yml"); got != "custom.yml" {
		t.Errorf("override not returned, got %q", got)
	}
}

func TestParseArgsHandlesEqualsAndBare(t *testing.T) {
	args := parseArgs([]string{"--json", "--ontology=custom.yml", "--strict"})
	if !boolFlag(args, "json") || !boolFlag(args, "strict") {
		t.Errorf("bare flags lost: %+v", args.flags)
	}
	if got := stringFlag(args, "ontology", ""); got != "custom.yml" {
		t.Errorf("--ontology=... lost: %q", got)
	}
}
