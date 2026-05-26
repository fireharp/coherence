package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs106To110() []Spec {
	return []Spec{
		{
			ID:             "ADV-106-github-action-local-uses-demo",
			Description:    "Add a GitHub Actions workflow that uses a missing local action; dangling_imports does not parse workflow uses operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: ".github/workflows/policy.yml",
				Content: `name: policy
on: [push]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./actions/missing-policy-check
`,
			},
		},
		{
			ID:             "ADV-107-terraform-module-source-demo",
			Description:    "Add a Terraform module that points at a missing local module path; dangling_imports does not parse Terraform sources.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "infra/main.tf",
				Content: `module "policy_gate" {
  source = "./modules/missing_policy_gate"
}
`,
			},
		},
		{
			ID:             "ADV-108-kustomize-resource-demo",
			Description:    "Add a Kustomize overlay referencing a missing resource file; dangling_imports does not parse Kustomize resources.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "deploy/kustomization.yaml",
				Content: `resources:
  - missing-policy-deployment.yaml
`,
			},
		},
		{
			ID:             "ADV-109-asciidoc-xref-demo",
			Description:    "Add an AsciiDoc xref to a missing local runbook; broken_links only scans Markdown inline links.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ops/policy.adoc",
				Content: "= Policy Ops\n\nSee xref:missing-runbook.adoc[missing runbook].\n",
			},
		},
		{
			ID:             "ADV-110-kotlin-ktor-route-demo",
			Description:    "Add a Kotlin Ktor route with no test coverage; orphan_endpoints does not extract Ktor route declarations.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "kotlin/src/main/kotlin/PolicyRoutes.kt",
				Content: `package example

import io.ktor.server.routing.Route
import io.ktor.server.routing.get

fun Route.policyRoutes() {
    get("/policy/audit") {
    }
}
`,
			},
		},
	}
}
