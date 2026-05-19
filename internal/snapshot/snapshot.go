// Package snapshot computes a Merkle snapshot of a repository — per-file
// content + semantic hashes plus directory roll-ups. See GOAL.md M2.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"coherence/internal/git"
)

// Kind is the per-file semantic classification.
type Kind string

const (
	KindMarkdown Kind = "markdown"
	KindYAML     Kind = "yaml"
	KindCode     Kind = "code"
	KindOther    Kind = "other"
)

// FileEntry captures the snapshot data for one tracked file.
type FileEntry struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Kind         Kind   `json:"kind"`
	ContentHash  string `json:"content_hash"`
	SemanticHash string `json:"semantic_hash"`
}

// DirectoryEntry is a Merkle roll-up node.
type DirectoryEntry struct {
	Path     string   `json:"path"`
	Hash     string   `json:"hash"`
	Children []string `json:"children"`
}

// Snapshot is the on-disk shape of `.coherence/snapshot.json`.
type Snapshot struct {
	GeneratedAt string           `json:"generated_at"`
	Files       []FileEntry      `json:"files"`
	Directories []DirectoryEntry `json:"directories"`
	RootHash    string           `json:"root_hash"`
	FileCount   int              `json:"file_count"`
}

// Path returns the canonical snapshot path for the given repo root.
func PathFor(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "snapshot.json")
}

// Compute walks the tracked file set (`git ls-files`) and produces a
// snapshot. Files that exist in the index but are missing on disk (e.g. just
// deleted) are skipped silently.
func Compute(rootDir string) (Snapshot, error) {
	tracked := git.LsFiles(rootDir)
	sort.Strings(tracked)

	files := make([]FileEntry, 0, len(tracked))
	for _, rel := range tracked {
		abs := filepath.Join(rootDir, rel)
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			continue
		}
		body, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		kind := classify(rel)
		content := sha256.Sum256(body)
		contentHash := hex.EncodeToString(content[:])
		semantic := contentHash
		ext := strings.ToLower(filepath.Ext(rel))
		switch {
		case kind == KindMarkdown:
			semantic = markdownSemantic(body)
		case ext == ".go":
			if h, ok := goSemantic(body); ok {
				semantic = h
			}
		default:
			if h, ok := codeSemantic(body, ext); ok {
				semantic = h
			}
		}
		files = append(files, FileEntry{
			Path:         rel,
			Size:         info.Size(),
			Kind:         kind,
			ContentHash:  contentHash,
			SemanticHash: semantic,
		})
	}

	dirs, rootHash := buildMerkle(files)

	return Snapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Files:       files,
		Directories: dirs,
		RootHash:    rootHash,
		FileCount:   len(files),
	}, nil
}

func classify(rel string) Kind {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".md", ".markdown":
		return KindMarkdown
	case ".yml", ".yaml":
		return KindYAML
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".sql", ".rb", ".rs", ".java", ".kt":
		return KindCode
	default:
		return KindOther
	}
}

// buildMerkle walks file entries top-down, building a directory hash for
// every prefix path. Directory hash = sha256 of sorted "name:child_hash\n"
// lines for direct children.
func buildMerkle(files []FileEntry) ([]DirectoryEntry, string) {
	type child struct {
		name string
		hash string
	}
	dirChildren := map[string][]child{}

	// Ensure dirChildren has an entry for every ancestor of every file,
	// even intermediate dirs whose direct children are all sub-directories.
	ensureAncestors := func(rel string) {
		d := path.Dir(rel)
		if d == "." {
			d = ""
		}
		for {
			if _, ok := dirChildren[d]; !ok {
				dirChildren[d] = nil
			}
			if d == "" {
				return
			}
			parent := path.Dir(d)
			if parent == "." {
				parent = ""
			}
			d = parent
		}
	}
	for _, f := range files {
		ensureAncestors(f.Path)
		dir := path.Dir(f.Path)
		if dir == "." {
			dir = ""
		}
		dirChildren[dir] = append(dirChildren[dir], child{
			name: path.Base(f.Path),
			hash: f.ContentHash,
		})
	}

	// Process directories deepest-first so parent dirs see child hashes.
	dirs := []string{}
	for d := range dirChildren {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return depth(dirs[i]) > depth(dirs[j])
	})

	dirHashes := map[string]string{}
	out := []DirectoryEntry{}
	for _, d := range dirs {
		kids := dirChildren[d]
		sort.Slice(kids, func(i, j int) bool { return kids[i].name < kids[j].name })

		h := sha256.New()
		names := make([]string, 0, len(kids))
		for _, k := range kids {
			h.Write([]byte(k.name))
			h.Write([]byte{':'})
			h.Write([]byte(k.hash))
			h.Write([]byte{'\n'})
			names = append(names, k.name)
		}
		dirHash := hex.EncodeToString(h.Sum(nil))
		dirHashes[d] = dirHash
		display := d
		if display == "" {
			display = "."
		}
		out = append(out, DirectoryEntry{
			Path:     display,
			Hash:     dirHash,
			Children: names,
		})
		if d != "" {
			parent := path.Dir(d)
			if parent == "." {
				parent = ""
			}
			dirChildren[parent] = append(dirChildren[parent], child{
				name: path.Base(d),
				hash: dirHash,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	root := dirHashes[""]
	if root == "" {
		root = hex.EncodeToString(sha256.New().Sum(nil))
	}
	return out, root
}

func depth(p string) int {
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// Write persists the snapshot to .coherence/snapshot.json.
func Write(rootDir string, s Snapshot) error {
	dst := PathFor(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(dst, buf, 0o644)
}
