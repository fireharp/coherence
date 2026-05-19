package graph

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"coherence/internal/git"
)

// Build walks the tracked file set and applies the MVP extractors to produce
// a graph. Provenance strings reference the file the edge/node came from.
func Build(rootDir string) (Graph, error) {
	b := NewBuilder()
	tracked := git.LsFiles(rootDir)

	// Pass 1: file + directory nodes + contains edges.
	for _, rel := range tracked {
		emitFileAndAncestors(b, rel)
	}

	// Pass 2: docs + frontmatter id nodes + mentions edges.
	trackedSet := map[string]struct{}{}
	for _, rel := range tracked {
		trackedSet[rel] = struct{}{}
	}
	for _, rel := range tracked {
		if !isMarkdown(rel) {
			continue
		}
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		emitDocNode(b, rel, data)
		emitFrontmatterIDNode(b, rel, data)
		emitMentionsEdges(b, rel, data, trackedSet)
		emitConceptNode(b, rel, data)
		emitClaimNodes(b, rel, data)
	}

	// Pass 3: ontology rules + commands.
	extractOntology(b, rootDir)

	// Pass 4: metric files (path-pattern driven).
	for _, rel := range tracked {
		if !isMetricFile(rel) {
			continue
		}
		emitMetricNode(b, rel)
	}

	// Pass 5: test files (path-pattern driven + verifies edge inference).
	for _, rel := range tracked {
		if !isTestFile(rel) {
			continue
		}
		emitTestNode(b, rel, trackedSet)
	}

	// Pass 6: evidence packets (docs/evidence/<bucket>/ convention).
	extractEvidence(b, tracked)

	// Pass 7: generated_artifact nodes from ontology rules' expect_any.
	extractGeneratedArtifacts(b, rootDir, tracked)

	// Pass 8: code_symbol nodes from a shallow Go AST scan.
	extractGoSymbols(b, rootDir, tracked)

	// Pass 9: data_model nodes from SQL/proto/GraphQL schema files.
	extractSchemas(b, rootDir, tracked)

	// Pass 10: command nodes from Makefile / *.mk target declarations.
	extractMakefile(b, rootDir, tracked)

	// Pass 11: code_symbol + depends_on from TypeScript shallow scan.
	extractTSSymbols(b, rootDir, tracked)

	// Pass 12: code_symbol + depends_on from Python shallow scan.
	extractPythonSymbols(b, rootDir, tracked)

	// Pass 13: command nodes from shell scripts (.sh/.bash/.zsh + shebang).
	extractShellCommands(b, rootDir, tracked)

	// Pass 14: code-level typed-id mentions (must run after Pass 2 so
	// the typed-id node set is complete).
	extractCodeMentions(b, rootDir, tracked)

	// Pass 15: code-level metric-name mentions (must run after Pass 4
	// so the metric node set is complete).
	extractMetricMentions(b, rootDir, tracked)

	// Pass 16: file-references via quoted path literals in code.
	extractFileReferences(b, rootDir, tracked)

	return b.Build(), nil
}

func isMetricFile(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".yaml" && ext != ".yml" {
		return false
	}
	// Match `rill/metrics/<name>.yaml`, `metrics/<name>.yaml`, and
	// `metrics/<sub>/.../*.yaml`. Tight enough to avoid false positives
	// from generic YAML files elsewhere in the repo.
	switch {
	case strings.HasPrefix(p, "rill/metrics/"):
		return true
	case strings.HasPrefix(p, "metrics/"):
		return true
	}
	return false
}

// emitMetricNode produces one metric node per metric file. The label is
// the filename without extension, slugified. Future iterations can deepen
// this to parse `measures: [...]` arrays and emit one node per measure.
func emitMetricNode(b *Builder, rel string) {
	base := filepath.Base(rel)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	slug := slugify(name)
	if slug == "" {
		return
	}
	b.AddNode(Node{
		ID:    MetricNodeID(slug),
		Kind:  NodeMetric,
		Label: name,
		Path:  rel,
	})
	b.AddEdge(Edge{
		From:       FileNodeID(rel),
		To:         MetricNodeID(slug),
		Kind:       EdgeDefines,
		Provenance: rel + " (metric file)",
	})
}

func isMarkdown(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func emitFileAndAncestors(b *Builder, rel string) {
	b.AddNode(Node{
		ID:    FileNodeID(rel),
		Kind:  NodeFile,
		Label: filepath.Base(rel),
		Path:  rel,
	})
	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}
	// Ancestor walk: every prefix becomes a directory node + contains edge.
	prev := ""
	for {
		var current string
		if dir == "" {
			current = ""
		} else {
			current = dir
		}
		b.AddNode(Node{
			ID:    DirNodeID(current),
			Kind:  NodeDirectory,
			Label: displayDir(current),
			Path:  current,
		})
		// Contains edge: current dir contains either prev (a deeper dir) or
		// the file itself on the first iteration.
		var childID string
		if prev == "" {
			childID = FileNodeID(rel)
		} else {
			childID = DirNodeID(prev)
		}
		b.AddEdge(Edge{From: DirNodeID(current), To: childID, Kind: EdgeContains})

		if current == "" {
			break
		}
		prev = current
		parent := path.Dir(current)
		if parent == "." {
			parent = ""
		}
		dir = parent
	}
}

func displayDir(p string) string {
	if p == "" {
		return "."
	}
	return p
}

func emitDocNode(b *Builder, rel string, data []byte) {
	title := docTitle(data, rel)
	b.AddNode(Node{
		ID:    DocNodeID(rel),
		Kind:  NodeDoc,
		Label: title,
		Path:  rel,
	})
	// `directory contains doc` edge is redundant with `directory contains
	// file`, but the doc node carries the structured title — wire it for
	// completeness so consumers can traverse dir→doc directly.
	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}
	b.AddEdge(Edge{From: DirNodeID(dir), To: DocNodeID(rel), Kind: EdgeContains})
}

var (
	frontmatterIDRe = regexp.MustCompile(`(?m)^id:\s*((US|ADR|IDR)-\d{3})\s*$`)
	// relationLineRe captures the four typed-id relation fields shipped
	// today: supersedes, contradicts, mirrors, invalidates. Accepts scalar
	// (`<key>: ADR-001`) and inline-list (`<key>: [ADR-001, ADR-002]`)
	// forms. The first capture group is the key name; the second is the
	// comma-joined value text.
	relationLineRe = regexp.MustCompile(`(?m)^(supersedes|contradicts|mirrors|invalidates):\s*(.+?)\s*$`)
	supersedesIDRe = regexp.MustCompile(`(US|ADR|IDR)-\d{3}`)
	headingRe      = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	// headingLeveledRe captures any markdown heading and exposes its level
	// (length of the leading `#` run). Used by concept extraction to emit
	// nodes for H1 + H2 while skipping H3+. Kept separate from
	// `headingRe` so `docTitle()` continues to use the H1-only matcher
	// for the title fallback.
	headingLeveledRe = regexp.MustCompile(`(?m)^(#+)\s+(.+)$`)
	titleFrontRe     = regexp.MustCompile(`(?m)^title:\s*(.+?)\s*$`)
	mdLinkRe         = regexp.MustCompile(`\[[^\]]*\]\(([^)\s#]+)(?:#[^)]*)?\)`)
	schemeRe         = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)
	slugStripRe      = regexp.MustCompile(`[^a-z0-9]+`)
	bulletRe         = regexp.MustCompile(`(?m)^\s*[-*+]\s+(.+)$`)
)

// claimVerbs is the closed set of leading modal/assertive verbs that mark a
// bullet item as a claim worth extracting. Tight set chosen to keep
// false-positive rate low — descriptive bullets ("- writes to disk") don't
// match.
var claimVerbs = map[string]bool{
	"must": true, "should": true, "shall": true,
	"requires": true, "require": true,
	"ensures": true, "ensure": true,
	"guarantees": true, "guarantee": true,
	"cannot": true, "will": true,
}

func docTitle(data []byte, rel string) string {
	if m := titleFrontRe.FindSubmatch(data); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	if m := headingRe.FindSubmatch(data); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return filepath.Base(rel)
}

func emitFrontmatterIDNode(b *Builder, rel string, data []byte) {
	var currentLabel, currentID string

	if matches := frontmatterIDRe.FindSubmatch(data); matches != nil {
		id := string(matches[1])
		label := string(matches[2])
		kind := idKindFromLabel(label)
		if kind == "" {
			return
		}
		b.AddNode(Node{
			ID:    IDNodeID(label, id),
			Kind:  kind,
			Label: id,
			Path:  rel,
		})
		b.AddEdge(Edge{
			From:       DocNodeID(rel),
			To:         IDNodeID(label, id),
			Kind:       EdgeDefines,
			Provenance: rel + " (frontmatter id)",
		})
		currentLabel, currentID = label, id
	} else {
		// Fallback: look at the filename for `US-###`, `ADR-###`, or `IDR-###`.
		base := filepath.Base(rel)
		for _, label := range []string{"US", "ADR", "IDR"} {
			labelRe := regexp.MustCompile(`\b` + label + `-\d{3}\b`)
			if m := labelRe.FindString(base); m != "" {
				kind := idKindFromLabel(label)
				if kind == "" {
					continue
				}
				b.AddNode(Node{
					ID:    IDNodeID(label, m),
					Kind:  kind,
					Label: m,
					Path:  rel,
				})
				b.AddEdge(Edge{
					From:       DocNodeID(rel),
					To:         IDNodeID(label, m),
					Kind:       EdgeDefines,
					Provenance: rel + " (filename)",
				})
				currentLabel, currentID = label, m
				break
			}
		}
	}
	if currentID == "" {
		return
	}
	emitSupersedesEdges(b, rel, data, currentLabel, currentID)
}

// emitSupersedesEdges scans the document's frontmatter for typed-id
// relation fields (`supersedes`, `contradicts`, `mirrors`, `invalidates`)
// and emits one edge per target id found, with the edge kind dispatched
// from the key name. Accepts scalar (`<key>: ADR-001`) and inline-list
// (`<key>: [ADR-001, ADR-002]`) forms. Target nodes are not pre-created —
// dangling references still emit edges so downstream consumers see the
// claim.
func emitSupersedesEdges(b *Builder, rel string, data []byte, currentLabel, currentID string) {
	from := IDNodeID(currentLabel, currentID)
	for _, m := range relationLineRe.FindAllSubmatch(data, -1) {
		key := string(m[1])
		value := string(m[2])
		var kind EdgeKind
		switch key {
		case "contradicts":
			kind = EdgeContradicts
		case "mirrors":
			kind = EdgeMirrors
		case "invalidates":
			kind = EdgeInvalidates
		default:
			kind = EdgeSupersedes
		}
		provLabel := key
		for _, idMatch := range supersedesIDRe.FindAllStringSubmatch(value, -1) {
			tgtLabel := idMatch[1]
			tgtID := idMatch[0]
			if tgtID == currentID {
				continue
			}
			b.AddEdge(Edge{
				From:       from,
				To:         IDNodeID(tgtLabel, tgtID),
				Kind:       kind,
				Provenance: rel + " (frontmatter " + provLabel + ")",
			})
		}
	}
}

func idKindFromLabel(label string) NodeKind {
	switch label {
	case "US":
		return NodeUserStory
	case "ADR":
		return NodeADR
	case "IDR":
		return NodeIDR
	}
	return ""
}

// emitConceptNode derives concepts from H1 + H2 markdown headings. Each
// captured heading emits one concept node + one `describes` edge from
// the source doc. H3+ are intentionally skipped — they typically denote
// sub-sub-topics that inflate the concept graph without adding
// meaningful coverage signal. Cross-doc dedup is unchanged: two docs
// whose headings slugify to the same value share one concept node.
// Per-doc dedup also applies — a doc with multiple H2s sharing a slug
// emits only one describes edge for that concept. Node meta carries
// `level` (`H1` / `H2`) for downstream filtering.
func emitConceptNode(b *Builder, rel string, data []byte) {
	seenInDoc := map[string]bool{}
	for _, m := range headingLeveledRe.FindAllSubmatch(data, -1) {
		level := len(m[1])
		if level > 2 {
			continue
		}
		label := strings.TrimSpace(string(m[2]))
		slug := slugify(label)
		if slug == "" {
			continue
		}
		if seenInDoc[slug] {
			continue
		}
		seenInDoc[slug] = true
		levelTag := "H" + string(rune('0'+level))
		b.AddNode(Node{
			ID:    ConceptNodeID(slug),
			Kind:  NodeConcept,
			Label: label,
			Meta:  map[string]string{"level": levelTag},
		})
		b.AddEdge(Edge{
			From:       DocNodeID(rel),
			To:         ConceptNodeID(slug),
			Kind:       EdgeDescribes,
			Provenance: rel + " (" + levelTag + ")",
		})
	}
}

// emitClaimNodes scans a markdown document for bullet lines beginning with
// an assertive verb (see claimVerbs) and emits one claim node per match.
// Claim IDs are content-addressed (sha256 prefix), so repeating the same
// claim text across multiple docs dedupes to one node with multiple defines
// edges — the wiring claim_support needs.
func emitClaimNodes(b *Builder, rel string, data []byte) {
	for _, m := range bulletRe.FindAllSubmatch(data, -1) {
		raw := strings.TrimSpace(string(m[1]))
		if raw == "" {
			continue
		}
		firstWord := strings.ToLower(strings.SplitN(raw, " ", 2)[0])
		// Strip trailing punctuation like "must:" → "must".
		firstWord = strings.TrimRight(firstWord, ".,;:!?")
		if !claimVerbs[firstWord] {
			continue
		}
		text := raw
		b.AddNode(Node{
			ID:    ClaimNodeID(text),
			Kind:  NodeClaim,
			Label: truncate(text, 120),
		})
		b.AddEdge(Edge{
			From:       DocNodeID(rel),
			To:         ClaimNodeID(text),
			Kind:       EdgeDefines,
			Provenance: rel + " (claim bullet)",
		})
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// slugify produces a lowercase hyphenated identifier suitable for concept
// node ids. Non-alphanumeric runs collapse to single hyphens; the result is
// trimmed of leading/trailing hyphens.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugStripRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func emitMentionsEdges(b *Builder, rel string, data []byte, trackedSet map[string]struct{}) {
	matches := mdLinkRe.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		raw := strings.TrimSpace(m[1])
		if raw == "" || schemeRe.MatchString(raw) || strings.HasPrefix(raw, "//") {
			continue
		}
		// Resolve relative paths.
		var target string
		if strings.HasPrefix(raw, "/") {
			target = strings.TrimPrefix(raw, "/")
		} else {
			target = path.Join(path.Dir(rel), raw)
		}
		target = path.Clean(target)
		if _, ok := trackedSet[target]; !ok {
			// Mentions edge is only useful when the target is tracked.
			continue
		}
		var toID string
		if isMarkdown(target) {
			toID = DocNodeID(target)
		} else {
			toID = FileNodeID(target)
		}
		b.AddEdge(Edge{
			From:       DocNodeID(rel),
			To:         toID,
			Kind:       EdgeMentions,
			Provenance: rel + " (markdown link)",
		})
	}
}

// isTestFile recognizes test-file path conventions across the main
// languages we extract today. Tight set chosen to avoid false positives
// on non-test code that happens to mention "test" in the filename.
func isTestFile(p string) bool {
	base := filepath.Base(p)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	switch ext {
	case ".go":
		return strings.HasSuffix(stem, "_test")
	case ".py":
		return strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test")
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		// foo.test.ts, foo.spec.ts
		secondaryExt := strings.ToLower(filepath.Ext(stem))
		return secondaryExt == ".test" || secondaryExt == ".spec"
	case ".rs":
		// Rust tests are typically under tests/ dir at the crate root,
		// not by filename pattern. Defer to directory check below.
	}
	// Directory-based fallbacks for the common test-folder conventions.
	pfx := func(s string) bool { return strings.HasPrefix(p, s) }
	if pfx("tests/") || pfx("test/") {
		switch ext {
		case ".py", ".ts", ".tsx", ".js", ".jsx", ".rs":
			return true
		}
	}
	if pfx("__tests__/") || strings.Contains(p, "/__tests__/") {
		switch ext {
		case ".ts", ".tsx", ".js", ".jsx":
			return true
		}
	}
	return false
}

// sourceFileForTest tries to reverse-map a test path to the source file it
// verifies. Returns (path, true) when a likely source exists in the
// tracked set. For Python tests/ and TS __tests__/ dirs the reverse-map
// is too ambiguous; we return false and skip the verifies edge.
func sourceFileForTest(testPath string, tracked map[string]struct{}) (string, bool) {
	dir := filepath.Dir(testPath)
	base := filepath.Base(testPath)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	switch ext {
	case ".go":
		if strings.HasSuffix(stem, "_test") {
			src := filepath.Join(dir, strings.TrimSuffix(stem, "_test")+".go")
			if _, ok := tracked[src]; ok {
				return src, true
			}
		}
	case ".py":
		var srcStem string
		switch {
		case strings.HasPrefix(stem, "test_"):
			srcStem = strings.TrimPrefix(stem, "test_")
		case strings.HasSuffix(stem, "_test"):
			srcStem = strings.TrimSuffix(stem, "_test")
		}
		if srcStem != "" {
			src := filepath.Join(dir, srcStem+".py")
			if _, ok := tracked[src]; ok {
				return src, true
			}
		}
	case ".ts", ".tsx", ".js", ".jsx":
		secondaryExt := strings.ToLower(filepath.Ext(stem))
		if secondaryExt == ".test" || secondaryExt == ".spec" {
			baseStem := strings.TrimSuffix(stem, secondaryExt)
			// Try the same extension first, then sibling variants.
			candidates := []string{
				filepath.Join(dir, baseStem+ext),
			}
			// .tsx → .ts fallback (and vice versa)
			switch ext {
			case ".tsx":
				candidates = append(candidates, filepath.Join(dir, baseStem+".ts"))
			case ".ts":
				candidates = append(candidates, filepath.Join(dir, baseStem+".tsx"))
			case ".jsx":
				candidates = append(candidates, filepath.Join(dir, baseStem+".js"))
			case ".js":
				candidates = append(candidates, filepath.Join(dir, baseStem+".jsx"))
			}
			for _, c := range candidates {
				if _, ok := tracked[c]; ok {
					return c, true
				}
			}
		}
	}
	return "", false
}

var (
	evidencePathRe  = regexp.MustCompile(`^docs/evidence/([^/]+)/`)
	typedIDBucketRe = regexp.MustCompile(`^(US|ADR|IDR)-\d{3}$`)
)

// extractEvidence walks tracked files, groups any under
// `docs/evidence/<bucket>/...` by bucket, and emits one evidence node per
// bucket. When the bucket name matches a typed-id pattern (`US-###`,
// `ADR-###`, `IDR-###`), wire a `supports` edge so the evidence is linked
// to the artifact it backs up.
func extractEvidence(b *Builder, tracked []string) {
	buckets := map[string]struct{}{}
	for _, rel := range tracked {
		m := evidencePathRe.FindStringSubmatch(rel)
		if m == nil {
			continue
		}
		buckets[m[1]] = struct{}{}
	}
	for bucket := range buckets {
		slug := slugify(bucket)
		if slug == "" {
			continue
		}
		b.AddNode(Node{
			ID:    EvidenceNodeID(slug),
			Kind:  NodeEvidence,
			Label: bucket,
			Path:  "docs/evidence/" + bucket,
		})
		if m := typedIDBucketRe.FindStringSubmatch(bucket); m != nil {
			label := m[1] // US / ADR / IDR
			b.AddEdge(Edge{
				From:       EvidenceNodeID(slug),
				To:         IDNodeID(label, bucket),
				Kind:       EdgeSupports,
				Provenance: "docs/evidence/" + bucket + " (typed-id bucket)",
			})
		}
	}
}

func emitTestNode(b *Builder, rel string, tracked map[string]struct{}) {
	b.AddNode(Node{
		ID:    TestNodeID(rel),
		Kind:  NodeTest,
		Label: filepath.Base(rel),
		Path:  rel,
	})
	if src, ok := sourceFileForTest(rel, tracked); ok {
		b.AddEdge(Edge{
			From:       TestNodeID(rel),
			To:         FileNodeID(src),
			Kind:       EdgeVerifies,
			Provenance: rel + " (test → source pair)",
		})
	}
}
