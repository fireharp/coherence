package adversarial

func embeddedMetricFrontendFiles() map[string]string {
	return map[string]string{
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
	}
}
