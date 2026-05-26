package adversarial

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadSpecs(opts Options) ([]Spec, error) {
	specs := BuiltinSpecs()
	if opts.TaxonomyPath != "" {
		extra, err := LoadTaxonomy(opts.TaxonomyPath, opts.RootDir)
		if err != nil {
			return nil, err
		}
		specs = append(specs, extra...)
	}
	if err := validateSpecs(specs); err != nil {
		return nil, err
	}
	return specs, nil
}

// LoadTaxonomy reads additional mutation specs from YAML or JSON.
func LoadTaxonomy(path, baseDir string) ([]Spec, error) {
	taxonomyPath := resolveFromBase(path, baseDir)
	data, err := os.ReadFile(taxonomyPath)
	if err != nil {
		return nil, err
	}
	var tf TaxonomyFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("taxonomy %s: %w", taxonomyPath, err)
	}
	if tf.Version == 0 {
		tf.Version = 1
	}
	if tf.Version != 1 {
		return nil, fmt.Errorf("taxonomy %s: unsupported version %d", taxonomyPath, tf.Version)
	}
	if len(tf.Mutation) == 0 {
		return nil, fmt.Errorf("taxonomy %s: mutations must not be empty", taxonomyPath)
	}
	return tf.Mutation, nil
}

func validateSpecs(specs []Spec) error {
	seen := map[string]bool{}
	for i, s := range specs {
		if s.ID == "" {
			return fmt.Errorf("mutation %d missing id", i)
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate mutation id %q", s.ID)
		}
		seen[s.ID] = true
		if !validOperations[s.Operation] {
			return fmt.Errorf("mutation %s: unsupported operation %q", s.ID, s.Operation)
		}
		if s.Operation != opBackdateHead && len(s.TargetKinds) == 0 {
			return fmt.Errorf("mutation %s: target_kinds must not be empty", s.ID)
		}
		for _, k := range s.TargetKinds {
			if !validNodeKinds[k] {
				return fmt.Errorf("mutation %s: unsupported target kind %q", s.ID, k)
			}
		}
		for _, m := range s.ExpectedMeters {
			if !validMeters[m] {
				return fmt.Errorf("mutation %s: unsupported expected meter %q", s.ID, m)
			}
		}
		for _, m := range s.AllowedSideEffectMeters {
			if !validMeters[m] {
				return fmt.Errorf("mutation %s: unsupported allowed side-effect meter %q", s.ID, m)
			}
		}
		if s.Selector.HasIncomingEdge != "" && !validEdgeKinds[s.Selector.HasIncomingEdge] {
			return fmt.Errorf("mutation %s: unsupported incoming edge kind %q", s.ID, s.Selector.HasIncomingEdge)
		}
		if s.Selector.HasOutgoingEdge != "" && !validEdgeKinds[s.Selector.HasOutgoingEdge] {
			return fmt.Errorf("mutation %s: unsupported outgoing edge kind %q", s.ID, s.Selector.HasOutgoingEdge)
		}
		for _, ext := range s.Selector.Extensions {
			if ext == "" || !strings.HasPrefix(ext, ".") {
				return fmt.Errorf("mutation %s: selector extensions must start with dot (got %q)", s.ID, ext)
			}
		}
		for _, env := range s.SkipConditions.RequireEnv {
			if strings.TrimSpace(env) == "" {
				return fmt.Errorf("mutation %s: skip_conditions.require_env must not contain empty names", s.ID)
			}
		}
		for _, p := range s.SkipConditions.RequireFiles {
			if !safeRelativePath(p) {
				return fmt.Errorf("mutation %s: skip_conditions.require_files must be relative in-repo paths (got %q)", s.ID, p)
			}
		}
		for _, engine := range s.SkipConditions.RequireOptionalEngines {
			if !validOptionalEngines[engine] {
				return fmt.Errorf("mutation %s: unsupported optional engine %q", s.ID, engine)
			}
		}
		for _, p := range []string{s.Edit.Path, s.Edit.NewPath} {
			if p != "" && !safeTemplatePath(p) {
				return fmt.Errorf("mutation %s: edit paths must be relative in-repo paths (got %q)", s.ID, p)
			}
		}
		if s.Operation == opReplaceText && s.Edit.Old == "" {
			return fmt.Errorf("mutation %s: replace_text requires edit.old", s.ID)
		}
		if s.Operation == opAppendText && s.Edit.Text == "" {
			return fmt.Errorf("mutation %s: append_text requires edit.text", s.ID)
		}
		if s.Operation == opRenameFile && s.Edit.NewPath == "" {
			return fmt.Errorf("mutation %s: rename_file requires edit.new_path", s.ID)
		}
		if s.Operation == opAddFile && s.Edit.Path == "" {
			return fmt.Errorf("mutation %s: add_file requires edit.path", s.ID)
		}
		if s.Operation == opRemoveLineContaining && s.Edit.LineContains == "" {
			return fmt.Errorf("mutation %s: remove_line_containing requires edit.line_contains", s.ID)
		}
	}
	return nil
}

func safeTemplatePath(p string) bool {
	replacer := strings.NewReplacer(
		"${target_id}", "target",
		"${target_path}", "target",
		"${target_label}", "target",
		"${target_dir}", "target",
		"${target_base}", "target",
		"${target_ext}", ".txt",
	)
	return safeRelativePath(replacer.Replace(p))
}

func safeRelativePath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if clean == "." || clean == ".." {
		return false
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	return !reservedRepoPath(clean)
}

func reservedRepoPath(clean string) bool {
	parts := strings.Split(clean, string(filepath.Separator))
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case ".git", ".coherence":
		return true
	default:
		return false
	}
}

func specIDs(specs []Spec) []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.ID)
	}
	sort.Strings(out)
	return out
}
