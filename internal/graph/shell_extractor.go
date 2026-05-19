package graph

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Shell extractor (Pass 13). Treats tracked shell scripts as commands:
// one `command:bash <relpath>` node per script, wired back to the
// source file via a `defines` edge. Coverage rules:
//
//   - Extension-based: `.sh`, `.bash`, `.zsh` always count as shells.
//   - Shebang fallback: extensionless tracked files whose first line is
//     `#!/usr/bin/env bash`, `#!/bin/bash`, `#!/bin/sh`, `#!/usr/bin/zsh`,
//     or any `#!.../sh` / `.../bash` / `.../zsh` variant. The detection
//     keeps the rule tight so non-shell scripts (Python, Node) don't
//     leak through.
//
// Recipe parsing (line-by-line shell parsing for invoked sub-commands)
// is intentionally deferred — Makefile-style "command-to-file output
// hints" remain a follow-up. The MVP surfaces existence + path,
// matching how Makefile targets are wired in Pass 10.

var shellExtensions = map[string]struct{}{
	".sh":   {},
	".bash": {},
	".zsh":  {},
}

// extractShellCommands is Pass 13.
func extractShellCommands(b *Builder, rootDir string, tracked []string) {
	for _, rel := range tracked {
		if !IsShellScript(rootDir, rel) {
			continue
		}
		cmd := "bash " + rel
		b.AddNode(Node{
			ID:    RuleNodeCommandID(cmd),
			Kind:  NodeCommand,
			Label: cmd,
			Path:  rel,
		})
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         RuleNodeCommandID(cmd),
			Kind:       EdgeDefines,
			Provenance: rel + " (shell script)",
		})
	}
}

// IsShellScript reports whether a tracked path is a shell script. The
// check is two-step: extension recognition (cheap, no file read), then
// shebang inspection for extensionless files. Files that fail both are
// skipped silently — non-shell scripts (Python, Node, etc.) never leak
// in.
func IsShellScript(rootDir, rel string) bool {
	ext := strings.ToLower(filepath.Ext(rel))
	if _, ok := shellExtensions[ext]; ok {
		return true
	}
	if ext != "" {
		// Has a non-shell extension — never a shell script for our
		// purposes (avoids false positives from .py/.go/.ts files
		// that happen to start with a shebang).
		return false
	}
	return hasShellShebang(filepath.Join(rootDir, rel))
}

// hasShellShebang reads the first line of a file and reports whether
// it starts with a shebang pointing at sh/bash/zsh. Reads at most 256
// bytes so it stays cheap even on large binaries that happen to live
// in the tree extensionless. Read failures are treated as "not a
// shell".
func hasShellShebang(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()
	br := bufio.NewReader(f)
	header, err := br.Peek(256)
	if err != nil && len(header) == 0 {
		return false
	}
	// First line only.
	line := header
	if nl := bytes.IndexByte(header, '\n'); nl >= 0 {
		line = header[:nl]
	}
	s := strings.TrimRight(string(line), " \r")
	if !strings.HasPrefix(s, "#!") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "#!"))
	// Handle `/usr/bin/env <interp>` and direct `/bin/<interp>` forms.
	if strings.HasPrefix(rest, "/usr/bin/env") {
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			return false
		}
		return isShellInterpName(parts[1])
	}
	// Direct path form: /bin/bash, /usr/local/bin/sh, etc. The
	// interpreter name is the basename of the first token.
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return false
	}
	return isShellInterpName(filepath.Base(parts[0]))
}

func isShellInterpName(s string) bool {
	switch s {
	case "sh", "bash", "zsh":
		return true
	}
	return false
}
