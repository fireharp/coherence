package adversarial

func embeddedDocFiles() map[string]string {
	return map[string]string{
		"docs/user-stories/US-001.md": `---
id: US-001
---
# US-001 Policy Approval
`,
		"docs/user-stories/US-003.md": `---
id: US-003
---
# US-003 Trace Coverage Story
`,
		"docs/evidence/US-001/proof.md": "# Evidence\n\nPolicy approval was reviewed for [US-001](../../user-stories/US-001.md).\n",
		"docs/evidence/US-003/proof.md": "Trace coverage was reviewed.\n1. Must retain audit evidence for every export. See [policy implementation](../../../pkg/policy/policy.go).\n\n### Deep Audit Evidence\n\nSee [deep policy source](../../../pkg/policy/policy.go).\n",
		"docs/specs/policy-source.md":   "# Policy Source\n\nThe policy threshold is 80. See [US-001](../user-stories/US-001.md).\n",
		"docs/specs/feature.md": `# Policy Feature

See [US-001](../user-stories/US-001.md), [policy source](../../pkg/policy/policy.go), and [threshold source](policy-source.md).

- Must require policy approval before order export.
`,
		"docs/specs/trace.md": `# Trace Coverage Spec

See [US-003](../user-stories/US-003.md).
See [policy](../../pkg/policy/policy.go).
`,
		"docs/decisions/ADR-001.md": `---
id: ADR-001
---
# ADR-001 Original Policy Decision

Backs [US-001](../user-stories/US-001.md).
`,
		"docs/decisions/ADR-050.md": `---
id: ADR-050
---
# ADR-050 Raw Citation Decision

Use the original raw-citation policy.
`,
		"docs/decisions/ADR-060.md": `---
id: ADR-060
---
# ADR-060 Reference Style Decision

Use the original reference-style decision.
`,
		"docs/runbook.md":             "# Runbook\n\nFollow [ADR-001](decisions/ADR-001.md) for policy decisions.\n",
		"docs/raw-runbook.md":         "# Raw Runbook\n\nFollow ADR-050 for raw-citation policy decisions.\n",
		"docs/ref-adr-runbook.md":     "# Reference ADR Runbook\n\nFollow [the reference-style ADR][adr-ref] for policy decisions.\n\n[adr-ref]: decisions/ADR-060.md\n",
		"docs/ref/index.md":           "# Reference Index\n\nSee [the target](target.md) and [[wiki-target.txt]].\n",
		"docs/ref/target.md":          "# Reference Target\n\nThis is the reference target for [US-001](../user-stories/US-001.md).\n",
		"docs/ref/wiki-target.txt":    "Wiki-style target for reference docs.\n",
		"docs/ref/refstyle.md":        "# Reference Style\n\nSee [the reference target][target-ref].\n\n[target-ref]: refstyle-target.md\n",
		"docs/ref/refstyle-target.md": "# Reference Style Target\n\nReference-style target for [US-001](../user-stories/US-001.md).\n",
		"docs/ref/html-link.md":       "# HTML Link\n\nSee <a href=\"html-target.md\">the HTML target</a>.\n",
		"docs/ref/html-target.md":     "# HTML Target\n\nHTML-link target for [US-001](../user-stories/US-001.md).\n",
		"docs/mdx/guide.mdx":          "# MDX Guide\n\nSee [the MDX target](target.txt).\n",
		"docs/mdx/MetricDemo.mdx":     "# Metric Demo\n\n<MetricCard metric=\"mdx_only\" />\n",
		"docs/mdx/target.txt":         "Target linked only from MDX.\n",
	}
}
