package templates

import (
	"strings"
	"testing"

	"coherence/internal/ontology"
)

func TestNamesIncludesEveryRequiredTemplate(t *testing.T) {
	want := []string{
		"generic", "go-cli", "typescript-app", "python-package",
		"data-pipeline", "docs-site", "infra-terraform", "monorepo", "agent-repo",
	}
	got := Names()
	gotSet := map[string]bool{}
	for _, n := range got {
		gotSet[n] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing template %q in %v", w, got)
		}
	}
}

func TestEveryTemplateOntologyParsesAndCarriesCommands(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			tpl, err := Resolve(name)
			if err != nil {
				t.Fatalf("resolve(%s): %v", name, err)
			}
			ont, err := ontology.Parse(tpl.Ontology, name+"/ontology.yml")
			if err != nil {
				t.Fatalf("parse(%s): %v", name, err)
			}
			if len(ont.Rules) == 0 {
				t.Fatalf("template %s has zero rules", name)
			}
			if len(ont.Commands) == 0 {
				t.Errorf("template %s has empty commands: map", name)
			}
			for _, r := range ont.Rules {
				if len(r.SuggestedCommands) == 0 {
					t.Errorf("template %s rule %q has no suggested_commands", name, r.ID)
				}
			}
			if len(tpl.PreCommitHook) == 0 || !strings.Contains(string(tpl.PreCommitHook), "coherence scan --staged") {
				t.Errorf("template %s missing or wrong pre-commit hook", name)
			}
			if len(tpl.SkillFiles) == 0 {
				t.Errorf("template %s missing coherence skill files", name)
			}
		})
	}
}

func TestSharedCoherenceSkillFiles(t *testing.T) {
	tpl, err := Resolve("generic")
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, f := range tpl.SkillFiles {
		paths[f.Path] = string(f.Data)
	}
	body := paths["SKILL.md"]
	if body == "" {
		t.Fatalf("missing SKILL.md in %+v", tpl.SkillFiles)
	}
	if !strings.Contains(body, "---\nname: coherence\n") {
		t.Fatalf("SKILL.md missing valid name frontmatter: %q", body)
	}
	if !strings.Contains(body, "description:") {
		t.Fatalf("SKILL.md missing description frontmatter: %q", body)
	}
	if paths["references/workflow.md"] == "" {
		t.Fatalf("missing references/workflow.md in %+v", tpl.SkillFiles)
	}
}

func TestResolveUnknownReturnsError(t *testing.T) {
	if _, err := Resolve("nonesuch"); err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestResolveEmptyUsesDefault(t *testing.T) {
	tpl, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != Default {
		t.Errorf("expected default %q, got %q", Default, tpl.Name)
	}
}
