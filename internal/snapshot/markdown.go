package snapshot

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// markdownLinkRe matches inline [text](target) and reference [text]: target
// forms. We keep only the target — link text counts as prose.
var (
	markdownInlineLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)`)
	markdownRefLinkRe    = regexp.MustCompile(`(?m)^\[[^\]]+\]:\s*(\S+)`)
)

// markdownSemantic returns a stable semantic-hash input for Markdown content.
// The canonical form includes:
//
//   - frontmatter (between leading `---` fences), verbatim with leading
//     whitespace trimmed,
//   - every ATX heading line (lines starting with `#`),
//   - every inline + reference link target,
//   - every fenced-code-block language label.
//
// Prose paragraphs are intentionally dropped: a typo in body text leaves the
// semantic hash unchanged. Editing a heading, a link target, a code fence
// language, or any frontmatter value changes it.
func markdownSemantic(body []byte) string {
	frontmatter, rest := splitFrontmatter(body)

	var b strings.Builder
	if len(frontmatter) > 0 {
		b.WriteString("frontmatter:\n")
		// Normalize each frontmatter line's trailing whitespace but keep the
		// content. Frontmatter is structured so we keep it all.
		for _, line := range strings.Split(string(frontmatter), "\n") {
			b.WriteString(strings.TrimRight(line, " \t"))
			b.WriteByte('\n')
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(string(rest)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inFence := false
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			lang := strings.TrimSpace(strings.TrimLeft(trimmed, "`~"))
			b.WriteString("fence:")
			b.WriteString(lang)
			b.WriteByte('\n')
			inFence = !inFence
			continue
		}
		if inFence {
			// We hash the body of code fences exactly because changing it
			// matters semantically (commands, code snippets) but we don't
			// want every retypo to flip it. Compromise: take the trimmed
			// line — collapses indentation but keeps content.
			b.WriteString("code:")
			b.WriteString(strings.TrimRight(raw, " \t"))
			b.WriteByte('\n')
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			b.WriteString("heading:")
			b.WriteString(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			b.WriteByte('\n')
		}
		for _, m := range markdownInlineLinkRe.FindAllStringSubmatch(raw, -1) {
			b.WriteString("link:")
			b.WriteString(m[1])
			b.WriteByte('\n')
		}
		for _, m := range markdownRefLinkRe.FindAllStringSubmatch(raw, -1) {
			b.WriteString("reflink:")
			b.WriteString(m[1])
			b.WriteByte('\n')
		}
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// splitFrontmatter returns (frontmatter, body) splitting on a leading `---`
// fence. If no frontmatter is present, the first return is nil.
func splitFrontmatter(body []byte) ([]byte, []byte) {
	if !startsWithFence(body) {
		return nil, body
	}
	// Find the closing fence.
	rest := body[3:]
	// skip optional newline after opening fence
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}
	// Search for "\n---\n" or "\n---<EOF>"
	for i := 0; i < len(rest); i++ {
		if rest[i] != '\n' {
			continue
		}
		j := i + 1
		// optional CR
		if j < len(rest) && rest[j] == '\r' {
			j++
		}
		if j+3 <= len(rest) && rest[j] == '-' && rest[j+1] == '-' && rest[j+2] == '-' {
			// must be followed by newline or EOF
			k := j + 3
			if k == len(rest) || rest[k] == '\n' || rest[k] == '\r' {
				frontmatter := rest[:i]
				bodyStart := k
				if bodyStart < len(rest) && rest[bodyStart] == '\n' {
					bodyStart++
				} else if bodyStart+1 < len(rest) && rest[bodyStart] == '\r' && rest[bodyStart+1] == '\n' {
					bodyStart += 2
				}
				return frontmatter, rest[bodyStart:]
			}
		}
	}
	return nil, body
}

func startsWithFence(body []byte) bool {
	if len(body) < 3 {
		return false
	}
	if body[0] != '-' || body[1] != '-' || body[2] != '-' {
		return false
	}
	if len(body) == 3 {
		return true
	}
	return body[3] == '\n' || body[3] == '\r'
}
