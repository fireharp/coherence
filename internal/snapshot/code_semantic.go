package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// codeSemantic computes a comment-and-whitespace-insensitive hash for
// TypeScript / JavaScript / Python source. Strips line comments,
// block / docstring comments, and collapses repeated whitespace.
//
// This is a regex-based approximation, not an AST pass — it's good
// enough that stale_tests and similar meters ignore comment-only and
// docstring-only edits, but it doesn't try to handle weird cases like
// comment delimiters appearing inside string literals. The fallback
// is content_hash (same string in same string out → same hash) so a
// confused strip is at worst as noisy as ContentHash, never less.
//
// Returns ok=false for languages this helper doesn't claim — callers
// should fall back to the content hash.
func codeSemantic(body []byte, ext string) (string, bool) {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".java", ".kt", ".rs", ".sql":
		return hashStripped(stripCStyle(string(body))), true
	case ".py", ".rb":
		return hashStripped(stripPython(string(body))), true
	}
	return "", false
}

var (
	cStyleLineRe  = regexp.MustCompile(`(?m)//[^\n]*`)
	cStyleBlockRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	pyLineRe      = regexp.MustCompile(`(?m)#[^\n]*`)
	pyTripleRe    = regexp.MustCompile(`(?s)"""[\s\S]*?"""|'''[\s\S]*?'''`)
	multiSpaceRe  = regexp.MustCompile(`[ \t]+`)
	multiBlankRe  = regexp.MustCompile(`\n\s*\n`)
)

func stripCStyle(s string) string {
	s = cStyleBlockRe.ReplaceAllString(s, "")
	s = cStyleLineRe.ReplaceAllString(s, "")
	return s
}

func stripPython(s string) string {
	s = pyTripleRe.ReplaceAllString(s, "")
	s = pyLineRe.ReplaceAllString(s, "")
	return s
}

func hashStripped(s string) string {
	s = multiSpaceRe.ReplaceAllString(s, " ")
	s = multiBlankRe.ReplaceAllString(s, "\n")
	s = strings.TrimSpace(s)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
