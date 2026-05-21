//go:build synthcorpus

// Part of evidence/synthetic — see src/main.go for the build-tag rationale.
package main

import "testing"

func TestSharedUtil(t *testing.T) {
	if sharedUtil(2) != 4 {
		t.Fatal("expected 4")
	}
}
