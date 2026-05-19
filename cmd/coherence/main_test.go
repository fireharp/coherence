package main

import (
	"strings"
	"testing"
)

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
