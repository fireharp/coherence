package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs126To130() []Spec {
	return []Spec{
		{
			ID:             "ADV-126-uppercase-story-frontmatter-demo",
			Description:    "Add a user story whose frontmatter key is uppercase ID; unimplemented_stories does not extract uppercase YAML keys.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unimplemented_stories"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/user-stories/story-uppercase.md",
				Content: `---
ID: US-126
---
# Uppercase Story
`,
			},
		},
		{
			ID:             "ADV-127-go-servemux-method-pattern-demo",
			Description:    "Add a Go 1.22 ServeMux method-pattern route with no test coverage; orphan_endpoints only sees http.Handle* or verb-named router methods.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/servemux.go",
				Content: `package policy

import "net/http"

func MountPolicyMux(mux *http.ServeMux) {
	mux.Handle("GET /policy/audit", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
}
`,
			},
		},
		{
			ID:             "ADV-128-protobuf-import-demo",
			Description:    "Add a protobuf schema importing a missing local proto; dangling_imports does not parse protobuf imports.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "schemas/policy.proto",
				Content: `syntax = "proto3";

import "missing_policy_options.proto";

message PolicyEvent {
  string id = 1;
}
`,
			},
		},
		{
			ID:             "ADV-129-markdown-task-list-claim-demo",
			Description:    "Add an unsupported task-list requirement; claim_support does not treat checkbox list items as claims.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"claim_support"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/specs/task-list-claim.md",
				Content: "# Task List Claim\n\n- [ ] Must retain approval logs for every export.\n",
			},
		},
		{
			ID:             "ADV-130-github-reusable-workflow-demo",
			Description:    "Add a workflow that calls a missing local reusable workflow; dangling_imports does not parse workflow-to-workflow uses paths.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: ".github/workflows/release.yml",
				Content: `name: release
on: [workflow_dispatch]
jobs:
  policy:
    uses: ./.github/workflows/missing-policy.yml
`,
			},
		},
	}
}
