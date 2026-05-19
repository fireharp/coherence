package graph

import "testing"

func TestMetricMentionsEmitsEdgeForQuotedName(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"rill/metrics/success_rate.yaml": "version: 1\nmeasures:\n  - name: success_rate\n",
		"frontend/dashboard.ts": `export const dash = { metric: "success_rate" }
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("frontend/dashboard.ts"),
		MetricNodeID("success-rate"), EdgeMentions) {
		t.Error("missing metric-mentions edge dashboard.ts → success-rate")
	}
}

func TestMetricMentionsHandlesSingleAndBacktickQuotes(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"rill/metrics/conv_rate.yaml": "version: 1\n",
		"src/app.py":                  `metric = 'conv_rate'`,
		"src/lib.ts":                  "const m = `conv_rate`",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"src/app.py", "src/lib.ts"} {
		if !hasEdge(g, FileNodeID(rel), MetricNodeID("conv-rate"), EdgeMentions) {
			t.Errorf("missing metric-mentions edge from %s", rel)
		}
	}
}

func TestMetricMentionsSkipsDefiningFile(t *testing.T) {
	// The yaml file that defines the metric shouldn't ALSO emit a
	// mentions edge — it already has `defines`. mentions on top would
	// be redundant noise.
	dir := gitInit(t, map[string]string{
		"rill/metrics/success_rate.yaml": "name: success_rate\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("rill/metrics/success_rate.yaml") {
			t.Errorf("defining file should not emit mentions: %+v", e)
		}
	}
}

func TestMetricMentionsSkipsMarkdown(t *testing.T) {
	// Markdown mentions are Pass 2's territory (markdown links). The
	// metric-mentions pass should skip *.md files entirely.
	dir := gitInit(t, map[string]string{
		"rill/metrics/success_rate.yaml": "name: success_rate\n",
		"docs/notes.md":                  "Run the \"success_rate\" check.\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, FileNodeID("docs/notes.md"),
		MetricNodeID("success-rate"), EdgeMentions) {
		t.Error("markdown should not emit metric-mention edges from Pass 15")
	}
}

func TestMetricMentionsSkipsUnknownNames(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"frontend/foo.ts": `const m = "unknown_metric"`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("frontend/foo.ts") {
			t.Errorf("unknown metric name should not emit edge: %+v", e)
		}
	}
}

func TestMetricMentionsDedupedPerFile(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"rill/metrics/success_rate.yaml": "name: success_rate\n",
		"frontend/app.ts":                `const a = "success_rate"; const b = 'success_rate'; const c = ` + "`success_rate`",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("frontend/app.ts") &&
			e.To == MetricNodeID("success-rate") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected one mentions edge per (file,metric), got %d", count)
	}
}

func TestMetricMentionsNoMetricsEmitsNothing(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"frontend/foo.ts": `const m = "irrelevant"`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("frontend/foo.ts") {
			t.Errorf("no metric nodes means no mentions edges, got %+v", e)
		}
	}
}
