package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs111To115() []Spec {
	return []Spec{
		{
			ID:             "ADV-111-commonjs-require-demo",
			Description:    "Add a plain CommonJS module requiring a missing local file; dangling_imports does not scan .cjs source.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "src/policy.cjs",
				Content: "const policy = require(\"./missing-policy\");\nmodule.exports = policy;\n",
			},
		},
		{
			ID:             "ADV-112-tsconfig-reference-demo",
			Description:    "Add a tsconfig project reference to a missing package; dangling_imports does not parse TypeScript project references.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "packages/app/tsconfig.json",
				Content: `{
  "extends": "../../tsconfig.json",
  "references": [
    { "path": "../missing-policy-package" }
  ]
}
`,
			},
		},
		{
			ID:             "ADV-113-nestjs-decorator-route-demo",
			Description:    "Add a NestJS decorator route with no test coverage; orphan_endpoints only scans call-expression route declarations.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/billing.controller.ts",
				Content: `import { Controller, Get } from "@nestjs/common";

@Controller("billing")
export class BillingController {
  @Get("invoices")
  invoices() {
    return [];
  }
}
`,
			},
		},
		{
			ID:             "ADV-114-helm-template-include-demo",
			Description:    "Add a Helm template that reads a missing local chart file; dangling_imports does not parse Helm template file references.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "charts/policy/templates/configmap.yaml",
				Content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: policy
data:
  policy.yaml: {{ .Files.Get "config/missing-policy.yaml" | quote }}
`,
			},
		},
		{
			ID:             "ADV-115-rst-local-link-demo",
			Description:    "Add a reStructuredText local link to a missing guide; broken_links only scans Markdown files.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/rst/index.rst",
				Content: "Policy Docs\n===========\n\nSee `missing guide <missing-guide.rst>`_.\n",
			},
		},
	}
}
