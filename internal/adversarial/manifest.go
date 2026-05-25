package adversarial

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadManifest parses a local corpus manifest. Relative repo paths resolve
// from baseDir so CLI calls behave predictably from the repo root.
func LoadManifest(path, baseDir string) (Manifest, error) {
	manifestPath := resolveFromBase(path, baseDir)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest %s: %w", manifestPath, err)
	}
	if m.Version == 0 {
		m.Version = 1
	}
	if m.Version != 1 {
		return Manifest{}, fmt.Errorf("manifest %s: unsupported version %d", manifestPath, m.Version)
	}
	if len(m.Repos) == 0 {
		return Manifest{}, fmt.Errorf("manifest %s: repos must not be empty", manifestPath)
	}
	for i := range m.Repos {
		r := &m.Repos[i]
		if r.ID == "" {
			return Manifest{}, fmt.Errorf("manifest %s: repo %d missing id", manifestPath, i)
		}
		if r.Path == "" {
			return Manifest{}, fmt.Errorf("manifest %s: repo %s missing path", manifestPath, r.ID)
		}
		if remoteLikePath(r.Path) {
			return Manifest{}, fmt.Errorf("manifest %s: repo %s path must be local, got %q", manifestPath, r.ID, r.Path)
		}
		if !filepath.IsAbs(r.Path) {
			r.Path = filepath.Join(baseDir, r.Path)
		}
		info, err := os.Stat(r.Path)
		if err != nil {
			return Manifest{}, fmt.Errorf("manifest %s: repo %s path %q: %w", manifestPath, r.ID, r.Path, err)
		}
		if !info.IsDir() {
			return Manifest{}, fmt.Errorf("manifest %s: repo %s path %q is not a directory", manifestPath, r.ID, r.Path)
		}
		if r.Weight <= 0 {
			r.Weight = 1
		}
		if len(r.Include) == 0 {
			r.Include = []string{"**"}
		}
		if len(r.Exclude) == 0 {
			r.Exclude = []string{".coherence/**", "vendor/**", "node_modules/**", ".git/**"}
		}
	}
	return m, nil
}

func resolveFromBase(path, baseDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func remoteLikePath(path string) bool {
	return strings.Contains(path, "://") || strings.HasPrefix(path, "git@")
}

func loadCorpus(opts Options) ([]corpusRepo, error) {
	if opts.ManifestPath == "" {
		return builtinCorpus(), nil
	}
	m, err := LoadManifest(opts.ManifestPath, opts.RootDir)
	if err != nil {
		return nil, err
	}
	out := make([]corpusRepo, 0, len(m.Repos))
	for _, r := range m.Repos {
		out = append(out, corpusRepo{RepoEntry: r})
	}
	return out, nil
}
