package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs142To145() []Spec {
	return []Spec{
		{
			ID:             "ADV-142-mkdocs-nav-missing-page-demo",
			Description:    "Add a MkDocs nav entry for a missing local page; broken_links only scans Markdown files.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "mkdocs.yml",
				Content: `site_name: Policy Docs
nav:
  - Policy Audit: docs/missing-policy-audit.md
`,
			},
		},
		{
			ID:             "ADV-143-docusaurus-sidebar-missing-doc-demo",
			Description:    "Add a Docusaurus sidebar pointing at a missing doc id; broken_links does not parse sidebar configs.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "sidebars.js",
				Content: `module.exports = {
  docs: [
    "policy/missing-audit",
  ],
};
`,
			},
		},
		{
			ID:             "ADV-144-nginx-include-dangling-demo",
			Description:    "Add an nginx include for a missing local config; dangling_imports does not parse nginx include directives.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "deploy/nginx.conf",
				Content: `events {}
http {
  include conf.d/missing-policy.conf;
}
`,
			},
		},
		{
			ID:             "ADV-145-systemd-environment-file-demo",
			Description:    "Add a systemd unit that references a missing EnvironmentFile; dangling_imports does not parse systemd unit file dependencies.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "deploy/policy.service",
				Content: `[Service]
EnvironmentFile=deploy/missing-policy.env
ExecStart=/usr/bin/policy-worker
`,
			},
		},
	}
}
