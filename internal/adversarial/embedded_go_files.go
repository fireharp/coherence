package adversarial

func embeddedGoFiles() map[string]string {
	return map[string]string{
		"pkg/policy/policy.go": `package policy

// Approve implements US-001.
func Approve(score int) bool {
	return threshold(score)
}

func threshold(score int) bool {
	return score >= 80
}

func CallerA(score int) bool { return Approve(score) }
func CallerB(score int) bool { return Approve(score + 1) }
func CallerC(score int) bool { return CallerA(score) || CallerB(score) }

// TracePolicy implements US-003.
func TracePolicy() bool { return true }
`,
		"pkg/policy/policy_test.go": `package policy

import "testing"

func TestApprove(t *testing.T) {
	if !Approve(81) {
		t.Fatal("expected approval")
	}
}
`,
		"pkg/risk/risk.go": `package risk

func Assess(score int) bool {
	return score >= 7
}
`,
		"pkg/risk/risk_integration_test.go": `package risk

import "testing"

func TestRiskIntegration(t *testing.T) {
	if !Assess(8) {
		t.Fatal("expected acceptable risk")
	}
}
`,
		"internal/a/a.go": `package a

import "example.com/adversarial/internal/b"

func A() { b.B() }
`,
		"internal/b/b.go": `package b

import "example.com/adversarial/internal/c"

func B() { c.C() }
`,
		"internal/c/c.go": `package c

func C() {}
`,
		"src/build-fixtures.go": `package main

func BuildFixtureVersion() string { return "v1" }
`,
		"fixtures/dashboard.json": "{\n  \"version\": \"v1\"\n}\n",
	}
}
