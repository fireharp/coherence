package adversarial

func embeddedBaseFiles() map[string]string {
	return map[string]string{
		"AGENTS.md": "# Agent Notes\n\nKeep docs, tests, metrics, and decisions coherent. See [US-001](docs/user-stories/US-001.md).\n",
		"go.mod":    "module example.com/adversarial\n\ngo 1.22\n",
		"Makefile": `include build/mk/policy.mk

check-policy:
	$(POLICY_CHECK)
`,
		"build/mk/policy.mk": "POLICY_CHECK=go test ./pkg/policy\n",
		"scripts/policy_check.sh": `#!/usr/bin/env bash
set -euo pipefail
source "./policy_lib.sh"
check_policy
`,
		"scripts/policy_lib.sh": `#!/usr/bin/env bash
check_policy() {
	go test ./pkg/policy
}
`,
		"tsconfig.json": `{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  }
}
`,
		"ontology.yml": `version: 1
optional_engines:
  callsite_blast_radius:
    enabled: true
    depth: 3
    max_symbols: 20
  dead_code:
    enabled: true
    max_items: 20
rules:
  - id: fixture-source-needs-output
    when: ["src/build-fixtures.go"]
    expect_any: ["fixtures/dashboard.json"]
    severity: error
    message: "Fixture source changed; regenerate fixtures/dashboard.json."
`,
	}
}
