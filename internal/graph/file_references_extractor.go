package graph

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// File-references extractor (Pass 16). Closes GOAL.md's "referenced
// files" extraction note: when a non-markdown tracked file contains a
// quoted string literal that looks path-like AND resolves to a tracked
// file, emit a `mentions` edge from the source file node to the
// referenced file node. The "must resolve to tracked" filter keeps
// false-positive noise minimal — random string literals that don't map
// to real repo files never emit edges.

// fileRefQuotedRe captures the contents of single, double, or
// backtick-quoted string literals. The path-like filter is applied
// after the match (in Go, not in the regex) so we can run two cheap
// checks: contains `/` OR has a small file extension.
var fileRefQuotedRe = regexp.MustCompile("[\"'`]([^\"'`\\s\\n]+)[\"'`]")

// fileRefExtensionRe matches a trailing `.<ext>` where <ext> is 1-5
// alpha chars. Used to gate filename-only references like
// `"config.json"` (no slash present).
var fileRefExtensionRe = regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9]{0,4}$`)

// extractFileReferences is Pass 16.
func extractFileReferences(b *Builder, rootDir string, tracked []string) {
	trackedSet := map[string]struct{}{}
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, m := range fileRefQuotedRe.FindAllStringSubmatch(string(data), -1) {
			lit := m[1]
			if !looksLikeFileRef(lit) {
				continue
			}
			target := resolveFileRef(rel, lit, trackedSet)
			if target == "" || target == rel {
				continue
			}
			if seen[target] {
				continue
			}
			seen[target] = true
			b.AddEdge(Edge{
				From:       FileNodeID(rel),
				To:         FileNodeID(target),
				Kind:       EdgeMentions,
				Provenance: rel + " (path literal " + lit + ")",
			})
		}
	}
}

// looksLikeFileRef applies a tight pre-filter so we only attempt to
// resolve strings that have a reasonable chance of being a path.
// Filters:
//   - reject absolute paths starting with `/` or `~`
//   - reject URLs (anything with `://`)
//   - require either a `/` OR a small extension
//   - reject excessively long strings (likely not paths)
//   - reject strings with whitespace (already filtered by the regex
//     but defensive)
func looksLikeFileRef(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") {
		return false
	}
	if strings.Contains(s, "://") {
		return false
	}
	if len(s) > 200 {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	hasSlash := strings.Contains(s, "/")
	hasExt := fileRefExtensionRe.MatchString(s)
	if !hasSlash && !hasExt {
		return false
	}
	return true
}

// resolveFileRef tries the literal against the tracked set, first
// relative to the source file's directory, then relative to repo root.
// Cleans the path before lookup so `./foo` and `foo` match the same
// tracked entry.
func resolveFileRef(fromRel, lit string, tracked map[string]struct{}) string {
	candidates := []string{}
	if strings.HasPrefix(lit, "./") || strings.HasPrefix(lit, "../") {
		// Relative-only: must resolve from the source dir.
		candidates = append(candidates, path.Clean(path.Join(path.Dir(fromRel), lit)))
	} else {
		// Try source-dir-relative first, then repo-root.
		candidates = append(candidates,
			path.Clean(path.Join(path.Dir(fromRel), lit)),
			path.Clean(lit),
		)
	}
	for _, c := range candidates {
		if _, ok := tracked[c]; ok {
			return c
		}
	}
	return ""
}
