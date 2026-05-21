package drift

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fireharp/coherence/internal/git"
)

// brokenLinkRe matches markdown inline links: `[text](target)`. Captures
// the target portion (everything inside the parens, up to a `#` anchor
// or whitespace).
var brokenLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s#]+)(?:#[^)]*)?\)`)

// brokenLinkSchemeRe matches scheme-prefixed targets (http:, https:, etc.)
// which we never flag as broken — those are external links.
var brokenLinkSchemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// computeBrokenLinks walks tracked `.md` files, regex-extracts inline
// links, resolves relative targets against the source file's directory,
// and lists those whose resolved path isn't present in the tracked set.
// External URLs (anything with a scheme prefix or starting with `//`) are
// skipped. Anchors-only links are skipped (no path to resolve).
func computeBrokenLinks(rootDir string) BrokenLinks {
	tracked := git.LsFiles(rootDir)
	trackedSet := map[string]bool{}
	for _, p := range tracked {
		trackedSet[p] = true
	}

	links := []BrokenLink{}
	for _, rel := range tracked {
		if filepath.Ext(rel) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, m := range brokenLinkRe.FindAllStringSubmatch(string(data), -1) {
			raw := strings.TrimSpace(m[1])
			if raw == "" {
				continue
			}
			if brokenLinkSchemeRe.MatchString(raw) || strings.HasPrefix(raw, "//") {
				continue
			}
			var target string
			if strings.HasPrefix(raw, "/") {
				target = strings.TrimPrefix(raw, "/")
			} else {
				target = path.Join(path.Dir(rel), raw)
			}
			target = path.Clean(target)
			if trackedSet[target] {
				continue
			}
			// Untracked-but-on-disk paths (e.g., `.gitignore`d LOCAL.md
			// notes the user references intentionally) aren't broken
			// for the user's working tree. Only flag links whose target
			// is missing from the filesystem entirely — those are real
			// 404s: typos, deletions, or stale references.
			if _, statErr := os.Stat(filepath.Join(rootDir, target)); statErr == nil {
				continue
			}
			key := rel + "|" + target
			if seen[key] {
				continue
			}
			seen[key] = true
			links = append(links, BrokenLink{Source: rel, Target: target})
		}
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Source != links[j].Source {
			return links[i].Source < links[j].Source
		}
		return links[i].Target < links[j].Target
	})
	if links == nil {
		links = []BrokenLink{}
	}
	return BrokenLinks{Score: len(links), Links: links}
}
