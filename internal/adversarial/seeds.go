package adversarial

func builtinCorpus() []corpusRepo {
	return []corpusRepo{
		{
			RepoEntry: RepoEntry{
				ID:      "embedded-agent-go-ts",
				Path:    "<embedded>",
				Tags:    []string{"agent-repo", "go", "typescript"},
				Weight:  1,
				Include: []string{"**"},
				Exclude: []string{".coherence/**", ".git/**"},
			},
			Files: embeddedAgentRepo(),
		},
	}
}

func embeddedAgentRepo() map[string]string {
	return map[string]string{
		"AGENTS.md": "# Agent Notes\n\nKeep docs, tests, metrics, and decisions coherent. See [US-001](docs/user-stories/US-001.md).\n",
		"go.mod":    "module example.com/adversarial\n\ngo 1.22\n",
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
		"pkg/policy/policy.go": `package policy

// Approve implements US-001.
func Approve(score int) bool {
	return threshold(score)
}

func threshold(score int) bool {
	return score >= 80
}

func CallerA(score int) bool { return Approve(score) }
func CallerB(score int) bool { return Approve(score + 1) }
func CallerC(score int) bool { return CallerA(score) || CallerB(score) }

// TracePolicy implements US-003.
func TracePolicy() bool { return true }
`,
		"pkg/policy/policy_test.go": `package policy

import "testing"

func TestApprove(t *testing.T) {
	if !Approve(81) {
		t.Fatal("expected approval")
	}
}
`,
		"pkg/risk/risk.go": `package risk

func Assess(score int) bool {
	return score >= 7
}
`,
		"pkg/risk/risk_integration_test.go": `package risk

import "testing"

func TestRiskIntegration(t *testing.T) {
	if !Assess(8) {
		t.Fatal("expected acceptable risk")
	}
}
`,
		"crates/risk/src/lib.rs": "pub fn risk_limit() -> i32 { 7 }\n",
		"tests/risk_limit_test.rs": `use risk::risk_limit;

#[test]
fn checks_limit() {
    assert_eq!(risk_limit(), 7);
}
`,
		"java/src/main/java/com/example/RiskPolicy.java": `package com.example;

public final class RiskPolicy {
    public static int limit() {
        return 7;
    }
}
`,
		"java/src/test/java/com/example/RiskPolicyTest.java": `package com.example;

import static org.junit.jupiter.api.Assertions.assertEquals;
import org.junit.jupiter.api.Test;

final class RiskPolicyTest {
    @Test
    void checksLimit() {
        assertEquals(7, RiskPolicy.limit());
    }
}
`,
		"csharp/RiskPolicy.cs": `namespace Example;

public static class RiskPolicy
{
    public static int Limit() => 7;
}
`,
		"csharp/RiskPolicyTests.cs": `using Xunit;

namespace Example.Tests;

public sealed class RiskPolicyTests
{
    [Fact]
    public void ChecksLimit()
    {
        Assert.Equal(7, RiskPolicy.Limit());
    }
}
`,
		"internal/a/a.go": `package a

import "example.com/adversarial/internal/b"

func A() { b.B() }
`,
		"internal/b/b.go": `package b

import "example.com/adversarial/internal/c"

func B() { c.C() }
`,
		"internal/c/c.go": `package c

func C() {}
`,
		"src/build-fixtures.go": `package main

func BuildFixtureVersion() string { return "v1" }
`,
		"fixtures/dashboard.json":    "{\n  \"version\": \"v1\"\n}\n",
		"metrics/signup_rate.yaml":   "version: 1\nmeasures:\n  - name: signup_rate\n",
		"metrics/churn_rate.yaml":    "version: 1\nmeasures:\n  - name: churn_rate\n",
		"metrics/revenue.yaml":       "version: 1\nmeasures:\n  - name: net_revenue\n",
		"metrics/vue_only.yaml":      "version: 1\nmeasures:\n  - name: vue_only\n",
		"metrics/mdx_only.yaml":      "version: 1\nmeasures:\n  - name: mdx_only\n",
		"metrics/svelte_only.yaml":   "version: 1\nmeasures:\n  - name: svelte_only\n",
		"metrics/yaml_only.yaml":     "version: 1\nmeasures:\n  - name: yaml_only\n",
		"metrics/toml_only.yaml":     "version: 1\nmeasures:\n  - name: toml_only\n",
		"metrics/template_only.yaml": "version: 1\nmeasures:\n  - name: template_only\n",
		"frontend/dashboard.ts":      "export const dashboard = { metric: \"signup_rate\" };\n",
		"frontend/splitMetric.ts":    "export const splitMetric = \"churn\" + \"_rate\";\nexport const dashboard = { metric: splitMetric };\n",
		"frontend/revenue.ts":        "export const revenueMetric = \"net_revenue\";\n",
		"frontend/templateMetric.ts": "const metricFamily = \"template\";\nexport const dashboard = { metric: `${metricFamily}_only` };\n",
		"frontend/MetricCard.vue":    "<script setup>\nconst metric = \"vue_only\";\n</script>\n<template>{{ metric }}</template>\n",
		"frontend/MetricBadge.svelte": `<script>
  export let metric = "svelte_only";
</script>

<span>{metric}</span>
`,
		"frontend/metric-config.yaml": "widgets:\n  - metric: yaml_only\n",
		"frontend/metric-config.toml": "[[widgets]]\nmetric = \"toml_only\"\n",
		"styles/app.css":              "@import \"./tokens.css\";\n.button { color: var(--brand); }\n",
		"styles/tokens.css":           ":root { --brand: #0369a1; }\n",
		"src/util.ts":                 "export const util = 1;\n",
		"src/usesUtil.ts":             "import { util } from './util';\nexport const value = util;\n",
		"src/reexported.ts":           "export const reexported = 1;\n",
		"src/barrel.ts":               "export { reexported } from './reexported';\nexport * from './reexported';\n",
		"src/lazy.ts":                 "export const lazy = 1;\n",
		"src/loadLazy.ts":             "export async function loadLazy() {\n  return import('./lazy');\n}\n",
		"src/cjsDep.ts":               "export const requiredValue = 1;\n",
		"src/requireConsumer.ts":      "const dep = require(\"./cjsDep\");\nexport const requiredValue = dep.requiredValue;\n",
		"src/importEqualsDep.ts":      "export const importEqualsValue = 1;\n",
		"src/importEqualsUser.ts":     "import dep = require(\"./importEqualsDep\");\nexport const importEqualsValue = dep.importEqualsValue;\n",
		"src/multilineDep.ts":         "export const multilineValue = 1;\n",
		"src/multilineImport.ts": `import {
  multilineValue,
} from "./multilineDep";

export const multilineImportValue = multilineValue;
`,
		"src/types.d.ts":   "export interface WidgetConfig { enabled: boolean }\n",
		"src/usesTypes.ts": "/// <reference path=\"./types.d.ts\" />\nexport const enabled = true;\n",
		"src/aliased.ts":   "export const aliased = 1;\n",
		"src/usesAlias.ts": "import { aliased } from '@/aliased';\nexport const aliasValue = aliased;\n",
		"src/widget.ts":    "export function widgetValue() {\n  return 1;\n}\n",
		"src/__tests__/widget.test.ts": `import { widgetValue } from "../widget";

test("widget value", () => {
  expect(widgetValue()).toBe(1);
});
`,
		"src/a/index.ts":           "import { b } from '../b';\nexport const a = b;\n",
		"src/b/index.ts":           "import { c } from '../c';\nexport const b = c;\n",
		"src/c/index.ts":           "export const c = 1;\n",
		"pyapp/__init__.py":        "",
		"pyapp/plugin.py":          "VALUE = 1\n",
		"pyapp/abs_plugin.py":      "value = 1\n",
		"pyapp/abs_consumer.py":    "from pyapp.abs_plugin import value\n\nresult = value\n",
		"pyapp/imported_module.py": "value = 1\n",
		"pyapp/import_consumer.py": "import pyapp.imported_module\n\nresult = pyapp.imported_module.value\n",
		"pyapp/dot_import_dep.py":  "value = 1\n",
		"pyapp/dot_import_consumer.py": `from . import dot_import_dep

result = dot_import_dep.value
`,
		"pyapp/calc.py":    "def calc_value():\n    return 1\n",
		"pyapp/cycle_a.py": "from .cycle_b import value_b\n\nvalue_a = value_b + 1\n",
		"pyapp/cycle_b.py": "value_b = 1\n",
		"tests/test_calc.py": `from pyapp.calc import calc_value

def test_calc_value():
    assert calc_value() == 1
`,
		"pyapp/loader.py": `import importlib

PLUGIN_MODULE = ".plugin"

def load_plugin():
    return importlib.import_module(PLUGIN_MODULE, __package__)
`,
		"src/api.ts": `const app = express();
app.get("/api/orders", getOrders);
function getOrders(req, res) { res.send("ok"); }
`,
		"src/api.test.ts": `import "./api";
test("orders endpoint", () => {
  expect(true).toBe(true);
});
`,
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
		"docs/evidence/US-003/proof.md": "Trace coverage was reviewed.\n",
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
