package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs070To080() []Spec {
	return []Spec{
		{
			ID:             "ADV-070-ts-test-dangling-import-demo",
			Description:    "Remove a TypeScript source imported only from a __tests__ file; dangling_imports skips test files.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/widget.ts"},
		},
		{
			ID:             "ADV-071-ruby-rails-route-demo",
			Description:    "Add an untested Rails-style route; endpoint extraction has no Ruby route parser.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "config/routes.rb",
				Content: `Rails.application.routes.draw do
  get "/api/rails-orders", to: "orders#index"
end
`,
			},
		},
		{
			ID:             "ADV-072-toml-typed-id-demo",
			Description:    "Add production TOML config with an unresolved typed ID; unknown_id_references sanitizes double-quoted string values.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "config/story-routing.toml",
				Content: `[routing]
fallback_story = "US-999"
`,
			},
		},
		{
			ID:             "ADV-073-toml-metric-alias-demo",
			Description:    "Rename a metric whose only stale alias is in a TOML dashboard config; orphaned_metric_aliases scans JSON but not TOML data files.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/toml_only.yaml"},
			Edit:           Edit{NewPath: "metrics/toml_only_v2.yaml"},
		},
		{
			ID:             "ADV-074-openapi-path-endpoint-demo",
			Description:    "Add an untested OpenAPI path; endpoint extraction currently only reads route declarations in source code.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "openapi.yaml",
				Content: `openapi: 3.1.0
info:
  title: Adversarial API
  version: "1.0"
paths:
  /api/openapi-orders:
    get:
      responses:
        "200":
          description: OK
`,
			},
		},
		{
			ID:             "ADV-075-java-stale-test-demo",
			Description:    "Change Java source covered by a JUnit test; stale_tests has no Java source-to-test reverse mapping.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "java/src/main/java/com/example/RiskPolicy.java"},
			Edit:           Edit{Old: "return 7;", New: "return 9;"},
		},
		{
			ID:                      "ADV-076-yaml-user-story-demo",
			Description:             "Add an unimplemented user story recorded as YAML; story extraction only treats Markdown docs as typed story nodes.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters:          []string{"unimplemented_stories"},
			AllowedSideEffectMeters: []string{"trace_coverage", "path_loss", "neighborhood_drift", "semantic_movement", "unknown_id_references"},
			Selector:                Selector{IDPrefix: "dir:docs/user-stories"},
			Edit: Edit{
				Path: "docs/user-stories/US-076.yaml",
				Content: `id: US-076
title: YAML Story
`,
			},
		},
		{
			ID:             "ADV-077-python-file-cycle-demo",
			Description:    "Close a Python import cycle within one package; dependency_cycles aggregates dependencies by directory.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dependency_cycles"},
			Selector:       Selector{PathGlob: "pyapp/cycle_b.py"},
			Edit: Edit{
				Old: "value_b = 1\n",
				New: "from .cycle_a import value_a\n\nvalue_b = value_a + 1\n",
			},
		},
		{
			ID:                      "ADV-078-markdown-angle-destination-space-demo",
			Description:             "Add a broken Markdown link whose destination is angle-bracket wrapped and contains a space; broken_links only parses bare no-whitespace inline targets.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ref/angle-destination-space.md",
				Content: "# Angle Destination Space\n\nSee [guide](<missing guide.md>).\n",
			},
		},
		{
			ID:             "ADV-079-python-from-dot-import-demo",
			Description:    "Remove a Python sibling module imported via `from . import sibling`; dangling_imports resolves the package rather than the imported name.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "pyapp/dot_import_dep.py"},
		},
		{
			ID:             "ADV-080-django-urlconf-endpoint-demo",
			Description:    "Add an untested Django URLConf route; endpoint extraction currently covers decorators and router calls but not urlpatterns.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pyapp/django_urls.py",
				Content: `from django.urls import path

def audit_events(request):
    return None

urlpatterns = [
    path("api/django-audit-events/", audit_events, name="audit-events"),
]
`,
			},
		},
	}
}
