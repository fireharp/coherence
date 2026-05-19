// Package templates ships the catalogue of `coherence init` starting points.
// Each template ships at minimum an ontology.yml; the pre-commit hook is
// shared across templates and lives under assets/_shared/pre-commit.
package templates

import (
	"embed"
	"fmt"
	"path"
	"sort"
)

//go:embed all:assets
var assets embed.FS

// Default is the template used when --template is not supplied.
const Default = "generic"

// Template bundles the files written by `init` for a given template name.
type Template struct {
	Name          string
	Ontology      []byte
	PreCommitHook []byte
}

// ScenariosFor returns the raw eval/scenarios.yml bytes for the named
// template, or an error if the template has no eval fixture.
func ScenariosFor(name string) ([]byte, error) {
	data, err := assets.ReadFile(path.Join("assets", name, "eval", "scenarios.yml"))
	if err != nil {
		return nil, fmt.Errorf("no eval scenarios for template %q", name)
	}
	return data, nil
}

// Names returns the sorted catalog of available template names.
func Names() []string {
	entries, err := assets.ReadDir("assets")
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "" || e.Name()[0] == '_' {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// Resolve returns the bundled assets for the named template, or an error if
// the name is unknown.
func Resolve(name string) (Template, error) {
	if name == "" {
		name = Default
	}
	ontologyPath := path.Join("assets", name, "ontology.yml")
	ontology, err := assets.ReadFile(ontologyPath)
	if err != nil {
		return Template{}, fmt.Errorf("unknown template %q (try one of: %v)", name, Names())
	}
	hook, err := assets.ReadFile("assets/_shared/pre-commit")
	if err != nil {
		return Template{}, fmt.Errorf("template assets missing shared pre-commit: %w", err)
	}
	return Template{
		Name:          name,
		Ontology:      ontology,
		PreCommitHook: hook,
	}, nil
}
