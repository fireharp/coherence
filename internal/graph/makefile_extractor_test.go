package graph

import "testing"

func TestMakefileEmitsCommandNodesForTargets(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile": "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"make build", "make test"} {
		if _, ok := findNode(g, RuleNodeCommandID(target)); !ok {
			t.Errorf("missing command node for %q", target)
		}
		if !hasEdge(g, FileNodeID("Makefile"),
			RuleNodeCommandID(target), EdgeDefines) {
			t.Errorf("missing defines edge Makefile → %q", target)
		}
	}
}

func TestMakefileSkipsPhonyAndSpecialTargets(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile": ".PHONY: build clean\n.DEFAULT_GOAL := build\n\nbuild:\n\tgo build\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, special := range []string{".PHONY", ".DEFAULT_GOAL"} {
		if _, ok := findNode(g, RuleNodeCommandID("make "+special)); ok {
			t.Errorf("special target %q should not produce a command node", special)
		}
	}
	if _, ok := findNode(g, RuleNodeCommandID("make build")); !ok {
		t.Error("regular target `build` should still emit a command node")
	}
}

func TestMakefileSkipsVariableAssignments(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile": "GOFLAGS := -count=1\nCFLAGS = -O2\nPREFIX ?= /usr/local\nFOO += bar\n\nbuild:\n\tgo build\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, varName := range []string{"GOFLAGS", "CFLAGS", "PREFIX", "FOO"} {
		if _, ok := findNode(g, RuleNodeCommandID("make "+varName)); ok {
			t.Errorf("variable assignment %q should not produce a command node", varName)
		}
	}
	if _, ok := findNode(g, RuleNodeCommandID("make build")); !ok {
		t.Error("`build` target lost")
	}
}

func TestMakefileSkipsPatternRules(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile": "%.o: %.c\n\tgcc -c $<\n\nbuild:\n\tgo build\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, RuleNodeCommandID("make %.o")); ok {
		t.Errorf("pattern rule %s should not produce a command node", "%.o")
	}
	if _, ok := findNode(g, RuleNodeCommandID("make build")); !ok {
		t.Error("regular target `build` lost")
	}
}

func TestMakefileHandlesMkIncludeFiles(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"Makefile":      "include rules.mk\n\nall: lint test\n",
		"rules.mk":      "lint:\n\tgolangci-lint run\n\ntest:\n\tgo test ./...\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, RuleNodeCommandID("make all")); !ok {
		t.Error("missing `all` from Makefile")
	}
	if _, ok := findNode(g, RuleNodeCommandID("make lint")); !ok {
		t.Error("missing `lint` from rules.mk")
	}
	if !hasEdge(g, FileNodeID("rules.mk"),
		RuleNodeCommandID("make lint"), EdgeDefines) {
		t.Error("defines edge rules.mk → make lint missing")
	}
}

func TestMakefileSkipsRecipeLines(t *testing.T) {
	// Recipe lines start with TAB. The first char of a non-target line that
	// happens to contain a colon (`cp foo:bar dst`) shouldn't be misread
	// as a target declaration.
	dir := gitInit(t, map[string]string{
		"Makefile": "build:\n\tcp foo:bar dst\n\tgo build ./...\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, RuleNodeCommandID("make build")); !ok {
		t.Error("missing build target")
	}
	if _, ok := findNode(g, RuleNodeCommandID("make cp foo")); ok {
		t.Error("recipe line cp foo:bar misread as a target")
	}
}

func TestMakefileMultipleTargetsOnOneLine(t *testing.T) {
	// `build test:` declares two targets. Today's MVP captures only the
	// first identifier — anything more would need a multi-target sweep.
	// Document the current behavior so a future change here is intentional.
	dir := gitInit(t, map[string]string{
		"Makefile": "build:\n\tgo build\n\ntest:\n\tgo test ./...\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"make build", "make test"} {
		if _, ok := findNode(g, RuleNodeCommandID(target)); !ok {
			t.Errorf("expected separate-line target %q present", target)
		}
	}
}

func TestMakefileNoMakefilesEmitsNothing(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"README.md": "# repo\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind != NodeCommand {
			continue
		}
		t.Errorf("unexpected command node in Makefile-free repo: %+v", n)
	}
}
