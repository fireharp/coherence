package main

import "testing"

func TestSharedUtil(t *testing.T) {
	if sharedUtil(2) != 4 {
		t.Fatal("expected 4")
	}
}
