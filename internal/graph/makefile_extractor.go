package graph

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// makefileNameSet recognizes the canonical Make entry-point filenames.
// Include `*.mk` separately via extension check; these three names are the
// portable entry-points GNU Make searches for by default.
var makefileNameSet = map[string]struct{}{
	"Makefile":    {},
	"makefile":    {},
	"GNUmakefile": {},
}

// targetLineRe matches a Makefile target declaration. Excludes lines that
// look like target-specific variable assignments (`target := value`) and
// double-colon rules (which are still targets — captured separately).
//
// Permitted target name chars: letters, digits, `_`, `.`, `-`, `/`, `+`.
// Trailing single colon optionally followed by `:` (double-colon) and
// dependency text. Variable assignments (`name = ...`, `name := ...`,
// `name ?= ...`, `name += ...`) are filtered by a pre-check.
var targetLineRe = regexp.MustCompile(`^([A-Za-z0-9_./+\-]+)::?(?:\s|$)`)

// varAssignRe filters out variable assignments before target detection.
// Match `name = ...`, `:=`, `?=`, `+=`, and `!=` forms; any of these
// disqualifies the line from being treated as a target.
var varAssignRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]+\s*(?::=|\?=|\+=|!=|=)`)

// isMakefilePath reports whether a tracked path looks like a Makefile.
// Recognizes the canonical three names anywhere in the tree plus any
// `*.mk` include file.
func isMakefilePath(rel string) bool {
	base := filepath.Base(rel)
	if _, ok := makefileNameSet[base]; ok {
		return true
	}
	if strings.EqualFold(filepath.Ext(base), ".mk") {
		return true
	}
	return false
}

// extractMakefile is Pass 10: scan tracked Makefiles for target
// declarations and emit one command:make <target> node per target, wired
// back to the source file via a defines edge. .PHONY targets and other
// special targets prefixed with `.` are skipped — they're meta-rules,
// not commands. Pattern rules (containing `%`) are also skipped since
// they're not directly runnable as `make <name>`.
func extractMakefile(b *Builder, rootDir string, tracked []string) {
	for _, rel := range tracked {
		if !isMakefilePath(rel) {
			continue
		}
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		emitMakefileTargets(b, rel, data)
	}
}

func emitMakefileTargets(b *Builder, rel string, data []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Allow long lines (some Makefiles have wide variable lists).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimLeft(line, " ")
		// Recipe lines start with TAB — never targets.
		if strings.HasPrefix(line, "\t") {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Variable assignments are not targets even though they may
		// contain `:` in some forms (`name:=value`).
		if varAssignRe.MatchString(trimmed) {
			continue
		}
		// Strip inline comments before the regex match so `target: # comment`
		// still parses cleanly.
		if i := strings.Index(trimmed, " #"); i > 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		m := targetLineRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		name := m[1]
		// Skip pattern rules and special targets like .PHONY, .DEFAULT_GOAL.
		if strings.HasPrefix(name, ".") {
			continue
		}
		if strings.Contains(name, "%") {
			continue
		}
		cmd := "make " + name
		b.AddNode(Node{
			ID:    RuleNodeCommandID(cmd),
			Kind:  NodeCommand,
			Label: cmd,
		})
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         RuleNodeCommandID(cmd),
			Kind:       EdgeDefines,
			Provenance: rel + " (makefile target)",
		})
	}
}
