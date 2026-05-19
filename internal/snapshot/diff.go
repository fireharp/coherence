package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Change types reported per-file in a DiffResult.
const (
	ChangeAdded          = "added"
	ChangeRemoved        = "removed"
	ChangeSemanticChange = "semantic_changed"
	ChangeSemanticNoop   = "semantic_noop"
)

// FileDiff describes one file's transition between two snapshots.
type FileDiff struct {
	Path             string `json:"path"`
	ChangeType       string `json:"change_type"`
	BaseContentHash  string `json:"base_content_hash,omitempty"`
	NewContentHash   string `json:"new_content_hash,omitempty"`
	BaseSemanticHash string `json:"base_semantic_hash,omitempty"`
	NewSemanticHash  string `json:"new_semantic_hash,omitempty"`
}

// DiffCounts is the diff tally.
type DiffCounts struct {
	Added           int `json:"added"`
	Removed         int `json:"removed"`
	SemanticChanged int `json:"semantic_changed"`
	SemanticNoop    int `json:"semantic_noop"`
}

// DiffResult is the shape of `.coherence/last-diff.json`.
type DiffResult struct {
	BaseRoot           string     `json:"base_root"`
	CurrentRoot        string     `json:"current_root"`
	RootChanged        bool       `json:"root_changed"`
	BaseGeneratedAt    string     `json:"base_generated_at"`
	CurrentGeneratedAt string     `json:"current_generated_at"`
	Files              []FileDiff `json:"files"`
	Counts             DiffCounts `json:"counts"`
}

// Diff returns the difference between two snapshots. Files are emitted in
// sorted order so the result is stable.
func Diff(base, current Snapshot) DiffResult {
	baseByPath := map[string]FileEntry{}
	for _, f := range base.Files {
		baseByPath[f.Path] = f
	}
	currentByPath := map[string]FileEntry{}
	for _, f := range current.Files {
		currentByPath[f.Path] = f
	}

	allPaths := map[string]struct{}{}
	for p := range baseByPath {
		allPaths[p] = struct{}{}
	}
	for p := range currentByPath {
		allPaths[p] = struct{}{}
	}
	sorted := make([]string, 0, len(allPaths))
	for p := range allPaths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	res := DiffResult{
		BaseRoot:           base.RootHash,
		CurrentRoot:        current.RootHash,
		RootChanged:        base.RootHash != current.RootHash,
		BaseGeneratedAt:    base.GeneratedAt,
		CurrentGeneratedAt: current.GeneratedAt,
	}
	for _, p := range sorted {
		b, hadBase := baseByPath[p]
		c, hadCur := currentByPath[p]
		switch {
		case !hadBase && hadCur:
			res.Files = append(res.Files, FileDiff{
				Path:            p,
				ChangeType:      ChangeAdded,
				NewContentHash:  c.ContentHash,
				NewSemanticHash: c.SemanticHash,
			})
			res.Counts.Added++
		case hadBase && !hadCur:
			res.Files = append(res.Files, FileDiff{
				Path:             p,
				ChangeType:       ChangeRemoved,
				BaseContentHash:  b.ContentHash,
				BaseSemanticHash: b.SemanticHash,
			})
			res.Counts.Removed++
		case b.ContentHash == c.ContentHash:
			// unchanged, skip
		default:
			change := ChangeSemanticChange
			if b.SemanticHash == c.SemanticHash {
				change = ChangeSemanticNoop
			}
			res.Files = append(res.Files, FileDiff{
				Path:             p,
				ChangeType:       change,
				BaseContentHash:  b.ContentHash,
				NewContentHash:   c.ContentHash,
				BaseSemanticHash: b.SemanticHash,
				NewSemanticHash:  c.SemanticHash,
			})
			if change == ChangeSemanticChange {
				res.Counts.SemanticChanged++
			} else {
				res.Counts.SemanticNoop++
			}
		}
	}
	if res.Files == nil {
		res.Files = []FileDiff{}
	}
	return res
}

// Load reads a JSON snapshot from disk.
func Load(path string) (Snapshot, error) {
	var s Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("snapshot %s: %w", path, err)
	}
	return s, nil
}

// DiffPath returns the canonical last-diff path for the given repo root.
func DiffPath(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "last-diff.json")
}

// WriteDiff persists a diff to .coherence/last-diff.json.
func WriteDiff(rootDir string, d DiffResult) error {
	dst := DiffPath(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(dst, buf, 0o644)
}

// HumanDiff renders a diff as concise readable lines.
func HumanDiff(d DiffResult) string {
	var b strings.Builder
	rootMark := "unchanged"
	if d.RootChanged {
		rootMark = "changed"
	}
	fmt.Fprintf(&b, "coherence diff: root %s\n", rootMark)
	fmt.Fprintf(&b, "  base    root=%s  generated=%s\n", short(d.BaseRoot), d.BaseGeneratedAt)
	fmt.Fprintf(&b, "  current root=%s  generated=%s\n", short(d.CurrentRoot), d.CurrentGeneratedAt)
	fmt.Fprintf(&b, "counts: added=%d removed=%d semantic_changed=%d semantic_noop=%d\n",
		d.Counts.Added, d.Counts.Removed, d.Counts.SemanticChanged, d.Counts.SemanticNoop)
	if len(d.Files) == 0 {
		fmt.Fprintln(&b, "(no per-file changes)")
		return b.String()
	}
	for _, f := range d.Files {
		fmt.Fprintf(&b, "  [%s] %s\n", f.ChangeType, f.Path)
	}
	return b.String()
}

func short(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}
