package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs131To135() []Spec {
	return []Spec{
		{
			ID:             "ADV-131-kotlin-stale-test-demo",
			Description:    "Change Kotlin source covered by a Kotlin test; stale_tests does not recognize Kotlin source/test mappings.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "kotlin/src/main/kotlin/RiskPolicy.kt"},
			Edit:           Edit{Old: "return 7", New: "return 9"},
		},
		{
			ID:             "ADV-132-laravel-route-demo",
			Description:    "Add a Laravel route with no test coverage; orphan_endpoints does not extract PHP route declarations.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "routes/web.php",
				Content: `<?php

use Illuminate\Support\Facades\Route;

Route::get('/policy/audit', function () {
    return 'ok';
});
`,
			},
		},
		{
			ID:             "ADV-133-openapi-local-ref-demo",
			Description:    "Add an OpenAPI schema with a missing local $ref file; dangling_imports does not parse OpenAPI references.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "openapi/policy.yaml",
				Content: `openapi: 3.1.0
info:
  title: Policy API
  version: "1"
paths: {}
components:
  schemas:
    Policy:
      $ref: "./schemas/missing-policy.yaml"
`,
			},
		},
		{
			ID:             "ADV-134-asciidoc-user-story-demo",
			Description:    "Add a user story in AsciiDoc; unimplemented_stories only sees Markdown/YAML story declarations.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unimplemented_stories"},
			AllowedSideEffectMeters: []string{
				"unknown_id_references",
			},
			Selector: Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/user-stories/agent-policy-story.adoc",
				Content: "= US-134 Agent Policy Story\n\nAgents must keep policy exports auditable.\n",
			},
		},
		{
			ID:             "ADV-135-gitlab-ci-include-demo",
			Description:    "Add a GitLab CI config including a missing local file; dangling_imports does not parse CI include graphs.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: ".gitlab-ci.yml",
				Content: `include:
  - local: ci/missing-policy.yml

policy:
  script: echo policy
`,
			},
		},
	}
}
