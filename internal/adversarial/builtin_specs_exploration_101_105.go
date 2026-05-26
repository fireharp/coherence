package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs101To105() []Spec {
	return []Spec{
		{
			ID:             "ADV-101-package-script-dangling-demo",
			Description:    "Add a package.json script that references a missing local Node script; dangling_imports does not parse package script operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "package.json",
				Content: `{
  "scripts": {
    "check:policy": "node scripts/missing-policy-check.js"
  }
}
`,
			},
		},
		{
			ID:             "ADV-102-go-embed-dangling-demo",
			Description:    "Add a Go source file with a go:embed directive for a missing local asset; dangling_imports does not parse embed operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/embed_policy.go",
				Content: `package policy

import _ "embed"

//go:embed templates/missing-policy.html
var policyTemplate string
`,
			},
		},
		{
			ID:             "ADV-103-compose-env-file-dangling-demo",
			Description:    "Add a Docker Compose service that references a missing env_file; dangling_imports does not parse Compose include operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docker-compose.yml",
				Content: `services:
  policy-worker:
    image: example/policy-worker:latest
    env_file:
      - .env.policy
`,
			},
		},
		{
			ID:             "ADV-104-bazel-load-dangling-demo",
			Description:    "Add a Bazel BUILD file that loads a missing Starlark rule file; dangling_imports does not parse Bazel load labels.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "BUILD.bazel",
				Content: `load("//tools:policy_rules.bzl", "policy_bundle")

policy_bundle(
    name = "policy_bundle",
    srcs = ["pkg/policy/policy.go"],
)
`,
			},
		},
		{
			ID:             "ADV-105-jupyter-import-dangling-demo",
			Description:    "Add a Jupyter notebook code cell that imports a missing helper module; dangling_imports does not parse notebook code cells.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "notebooks/policy_risk.ipynb",
				Content: `{
  "cells": [
    {
      "cell_type": "code",
      "metadata": {},
      "source": [
        "from helpers.risk_policy import score_policy\n",
        "score_policy({\"tier\": \"restricted\"})\n"
      ]
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
}
`,
			},
		},
	}
}
