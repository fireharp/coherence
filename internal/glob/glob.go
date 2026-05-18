// Package glob ports lib/glob.mjs: tiny glob -> regexp matcher used
// against forward-slash repo-relative paths.
//
//	**  -> matches any depth (including zero) when followed by /, else .*
//	*   -> matches any chars except /
//	?   -> matches one char except /
package glob

import (
	"regexp"
	"strings"
	"sync"
)

var (
	cacheMu sync.Mutex
	cache   = map[string]*regexp.Regexp{}
)

var regexMeta = ".+^$(){}|\\"

func escape(c byte) string {
	if strings.IndexByte(regexMeta, c) >= 0 {
		return "\\" + string(c)
	}
	return string(c)
}

func toRegex(pattern string) *regexp.Regexp {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if re, ok := cache[pattern]; ok {
		return re
	}
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		c := pattern[i]
		switch {
		case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			if i+2 < len(pattern) && pattern[i+2] == '/' {
				b.WriteString("(?:.*/)?")
				i += 3
			} else {
				b.WriteString(".*")
				i += 2
			}
		case c == '*':
			b.WriteString("[^/]*")
			i++
		case c == '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(escape(c))
			i++
		}
	}
	b.WriteByte('$')
	re := regexp.MustCompile(b.String())
	cache[pattern] = re
	return re
}

// Match reports whether path satisfies the glob pattern.
func Match(pattern, path string) bool {
	return toRegex(pattern).MatchString(path)
}

// AnyMatches reports whether at least one path matches at least one glob.
func AnyMatches(globs, paths []string) bool {
	for _, g := range globs {
		re := toRegex(g)
		for _, p := range paths {
			if re.MatchString(p) {
				return true
			}
		}
	}
	return false
}

// FilesMatching returns the subset of paths that match at least one glob,
// preserving the input order.
func FilesMatching(globs, paths []string) []string {
	out := make([]string, 0)
	for _, p := range paths {
		for _, g := range globs {
			if toRegex(g).MatchString(p) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// TriggeredGlobs returns the subset of globs that match at least one path.
func TriggeredGlobs(globs, paths []string) []string {
	out := make([]string, 0)
	for _, g := range globs {
		re := toRegex(g)
		for _, p := range paths {
			if re.MatchString(p) {
				out = append(out, g)
				break
			}
		}
	}
	return out
}
