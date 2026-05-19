package graph

import "testing"

func TestShellEmitsCommandFromShExtension(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"scripts/build.sh": "#!/bin/sh\nset -e\necho build\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := RuleNodeCommandID("bash scripts/build.sh")
	if _, ok := findNode(g, id); !ok {
		t.Error("missing command node for scripts/build.sh")
	}
	if !hasEdge(g, FileNodeID("scripts/build.sh"), id, EdgeDefines) {
		t.Error("missing defines edge from scripts/build.sh")
	}
}

func TestShellEmitsCommandForBashAndZsh(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.bash": "#!/bin/bash\necho a\n",
		"b.zsh":  "#!/usr/bin/zsh\necho b\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"a.bash", "b.zsh"} {
		id := RuleNodeCommandID("bash " + rel)
		if _, ok := findNode(g, id); !ok {
			t.Errorf("missing command node for %s", rel)
		}
	}
}

func TestShellShebangDetectionOnExtensionlessFile(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"bin/run":    "#!/usr/bin/env bash\necho run\n",
		"bin/run-sh": "#!/bin/sh\necho run\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"bin/run", "bin/run-sh"} {
		id := RuleNodeCommandID("bash " + rel)
		if _, ok := findNode(g, id); !ok {
			t.Errorf("shebang fallback failed for %s", rel)
		}
	}
}

func TestShellShebangIgnoresNonShellInterps(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"bin/runpy":   "#!/usr/bin/env python\nprint('hi')\n",
		"bin/runnode": "#!/usr/bin/env node\nconsole.log('hi')\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"bin/runpy", "bin/runnode"} {
		if _, ok := findNode(g, RuleNodeCommandID("bash "+rel)); ok {
			t.Errorf("non-shell shebang should not emit shell command for %s", rel)
		}
	}
}

func TestShellNoShellFilesEmitsNothing(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"README.md": "# repo\n",
		"main.go":   "package main\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCommand && (n.Label == "bash README.md" || n.Label == "bash main.go") {
			t.Errorf("non-shell file should not emit shell command: %+v", n)
		}
	}
}

func TestShellExtensionTrumpsExtensionlessShebangCheck(t *testing.T) {
	// `.py` extension means even with a bash shebang we never treat
	// it as shell — the shebang check is for extensionless files only.
	dir := gitInit(t, map[string]string{
		"weird.py": "#!/bin/bash\necho hi # supposedly python but starts with bash shebang\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, RuleNodeCommandID("bash weird.py")); ok {
		t.Error("non-shell extension should never emit shell command, regardless of shebang")
	}
}

func TestShellAllowsMixedShellAndMakefile(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile":         "build:\n\t./scripts/build.sh\n",
		"scripts/build.sh": "#!/bin/sh\necho build\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, RuleNodeCommandID("make build")); !ok {
		t.Error("makefile target missing")
	}
	if _, ok := findNode(g, RuleNodeCommandID("bash scripts/build.sh")); !ok {
		t.Error("shell script missing")
	}
}
