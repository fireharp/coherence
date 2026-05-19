package drift

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"coherence/internal/git"
	"coherence/internal/graph"
)

// typedIDInCodeRe finds US-###, ADR-###, IDR-### tokens anywhere in a
// non-Markdown file's content. Word-boundary anchors keep `US-001-foo`
// out of the match set.
var typedIDInCodeRe = regexp.MustCompile(`\b(US|ADR|IDR)-\d{3}\b`)

// backtickSpanRe matches anything between a pair of backticks (including
// across newlines). Backticks demarcate inline-code in doc comments and
// raw-string literals in Go / template literals in TS/JS. IDs found
// inside such spans are documentation examples or embedded fixture
// data, not real references — skip them.
var backtickSpanRe = regexp.MustCompile("(?s)`[^`]*`")

// positionInsideBacktickSpan reports whether byte offset pos in content
// falls inside any backtick-delimited span. Used to filter out typed-id
// mentions that are really inline-code examples (e.g., `US-001` in a
// Go doc comment) or embedded fixtures (e.g., samples.go raw-string
// literals containing fake US-### ids).
//
// Backticks that live inside a `"..."` double-quoted string literal
// (e.g., a Go regex literal `"(?s)`[^`]*`"`) are not real delimiters
// and would mis-pair the rest of the file — neutralize them first by
// blanking out double-quoted content while keeping byte offsets stable.
func positionInsideBacktickSpan(content []byte, pos int) bool {
	stripped := blankDoubleQuoted(content)
	for _, span := range backtickSpanRe.FindAllIndex(stripped, -1) {
		if pos >= span[0] && pos < span[1] {
			return true
		}
	}
	return false
}

// blankDoubleQuoted returns a copy of content with the *contents* of
// `"..."` spans replaced by spaces. Opening/closing quotes and byte
// offsets are preserved. Escaped quotes (\") inside a string are
// honored. Used to neutralize backticks that live inside Go string
// literals before backtick-pair scanning.
func blankDoubleQuoted(content []byte) []byte {
	out := make([]byte, len(content))
	inQuote := false
	escape := false
	for i, c := range content {
		out[i] = c
		if escape {
			escape = false
			if inQuote {
				out[i] = ' '
			}
			continue
		}
		if c == '\\' && inQuote {
			escape = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			out[i] = ' '
		}
	}
	return out
}

// positionInsideSameLineQuotes reports whether byte offset pos sits
// inside a `"..."` (or `'...'`) span on its own line. This catches
// typed-id mentions that are string-literal sample data — e.g.,
// `"docs/user-stories/US-007.md"` in a fixture map — without false-
// positives across multi-line constructs.
func positionInsideSameLineQuotes(content []byte, pos int) bool {
	lineStart := pos
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	dq, sq := 0, 0
	for i := lineStart; i < pos; i++ {
		c := content[i]
		// Don't count escaped quotes (preceding char is backslash and
		// that backslash itself isn't escaped). Keep this simple — the
		// common case dominates.
		if (c == '"' || c == '\'') && (i == lineStart || content[i-1] != '\\') {
			if c == '"' {
				dq++
			} else {
				sq++
			}
		}
	}
	return dq%2 == 1 || sq%2 == 1
}

// fixtureDirSegments lists path segments whose contents are almost
// always test fixtures across languages and frameworks. Files under
// any of these directories are excluded from unknown_id_references —
// the meter is meant to catch production-code references to
// unresolvable typed-ids, not fixture data that uses fake ids on
// purpose.
var fixtureDirSegments = []string{
	"scenarios/",
	"fixtures/",
	"testdata/",
	"golden/",
	"eval/",
}

func isFixturePath(rel string) bool {
	// Normalize to forward-slash for the substring check on Windows
	// (LsFiles returns POSIX paths, but be defensive).
	p := "/" + strings.ReplaceAll(rel, "\\", "/")
	for _, seg := range fixtureDirSegments {
		if strings.Contains(p, "/"+seg) {
			return true
		}
	}
	return false
}

// computeUnknownIDReferences walks tracked non-Markdown files looking
// for typed-id mentions. Each mention is checked against the graph's
// known typed-id nodes (user_story / adr / idr). Unmatched references
// are flagged. Markdown is skipped because docs frequently mention
// not-yet-implemented or planned ids deliberately — the IDs scanner
// upstream of this meter only validated additions in non-Markdown files
// for the same reason.
func computeUnknownIDReferences(rootDir string, g graph.Graph) UnknownIDReferences {
	known := map[string]bool{}
	for _, n := range g.Nodes {
		switch n.Kind {
		case graph.NodeUserStory, graph.NodeADR, graph.NodeIDR:
			known[n.ID] = true
		}
	}

	tracked := git.LsFiles(rootDir)
	refs := []UnknownIDReference{}
	seen := map[string]bool{}
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			continue
		}
		// Test files frequently use fake typed-ids as fixtures
		// (mocked spec data, golden inputs). Excluding them here
		// keeps the meter focused on production code that references
		// an id with no defining doc.
		if graph.IsTestFile(rel) {
			continue
		}
		// Agent-skill metadata (Codex / Claude skill packages) is
		// fixture-heavy by design. Skip the .agents/ prefix used by
		// the install conventions.
		if strings.HasPrefix(rel, ".agents/") {
			continue
		}
		// Common test-fixture directory conventions across languages
		// and frameworks. Files living in these dirs typically use
		// fake typed-ids for test purposes (golden inputs, scenario
		// fixtures, mocked spec data) and shouldn't contribute to the
		// production-code signal.
		if isFixturePath(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		for _, m := range typedIDInCodeRe.FindAllSubmatchIndex(data, -1) {
			start := m[0]
			end := m[1]
			if positionInsideBacktickSpan(data, start) {
				continue
			}
			if positionInsideSameLineQuotes(data, start) {
				continue
			}
			label := string(data[m[2]:m[3]])
			id := string(data[start:end])
			nodeID := strings.ToLower(label) + ":" + id
			if known[nodeID] {
				continue
			}
			key := rel + "|" + nodeID
			if seen[key] {
				continue
			}
			seen[key] = true
			var kind string
			switch label {
			case "US":
				kind = "user_story"
			case "ADR":
				kind = "adr"
			case "IDR":
				kind = "idr"
			}
			refs = append(refs, UnknownIDReference{File: rel, ID: id, Kind: kind})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].ID < refs[j].ID
	})
	return UnknownIDReferences{Score: len(refs), UnknownRefs: refs}
}
