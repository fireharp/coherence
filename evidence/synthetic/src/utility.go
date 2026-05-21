//go:build synthcorpus

// Part of evidence/synthetic — see src/main.go for the build-tag rationale.
package main

// GT: live
func sharedUtil(x int) int {
	return x * 2
}

// GT: dead-code
func unusedUtil(s string) string {
	return s + "!"
}
