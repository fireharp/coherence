package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fireharp/coherence/internal/graph"
)

func metricGitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOrphanedMetricsNoBaselineSilent(t *testing.T) {
	r := computeOrphanedMetricAliases(t.TempDir(), nil, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected silent without baseline, got %d", r.Score)
	}
	if r.Orphans == nil {
		t.Error("Orphans should be []OrphanedMetricAlias{}")
	}
}

func TestOrphanedMetricsDetectsFrontendReferenceToRemoved(t *testing.T) {
	dir := metricGitInit(t, map[string]string{
		"frontend/dash.ts": `export const m = "success_rate";`,
	})
	base := &graph.Graph{Nodes: []graph.Node{
		{ID: "metric:success-rate", Kind: graph.NodeMetric, Label: "success_rate"},
	}}
	current := graph.Graph{Nodes: []graph.Node{
		{ID: "metric:conversion-rate", Kind: graph.NodeMetric, Label: "conversion_rate"},
	}}
	r := computeOrphanedMetricAliases(dir, base, current)
	if r.Score != 1 || r.Orphans[0].OrphanName != "success_rate" {
		t.Errorf("expected success_rate orphan, got %+v", r)
	}
	if r.Orphans[0].File != "frontend/dash.ts" {
		t.Errorf("unexpected file: %+v", r.Orphans[0])
	}
}

func TestOrphanedMetricsNoOrphanWhenNamePreserved(t *testing.T) {
	dir := metricGitInit(t, map[string]string{
		"frontend/dash.ts": `export const m = "success_rate";`,
	})
	base := &graph.Graph{Nodes: []graph.Node{
		{ID: "metric:success-rate", Kind: graph.NodeMetric, Label: "success_rate"},
	}}
	// current still has the metric — no orphan.
	current := graph.Graph{Nodes: []graph.Node{
		{ID: "metric:success-rate", Kind: graph.NodeMetric, Label: "success_rate"},
	}}
	r := computeOrphanedMetricAliases(dir, base, current)
	if r.Score != 0 {
		t.Errorf("expected zero orphans, got %+v", r)
	}
}

func TestOrphanedMetricsSkipsNonFrontendFiles(t *testing.T) {
	dir := metricGitInit(t, map[string]string{
		"docs/notes.md":    `Reference to success_rate metric.`,
		"internal/main.go": `package main; const M = "success_rate"`,
	})
	base := &graph.Graph{Nodes: []graph.Node{
		{ID: "metric:success-rate", Kind: graph.NodeMetric, Label: "success_rate"},
	}}
	current := graph.Graph{}
	r := computeOrphanedMetricAliases(dir, base, current)
	if r.Score != 0 {
		t.Errorf("non-frontend mentions should be ignored, got %+v", r.Orphans)
	}
}

func TestOrphanedMetricsDedupsPerFileName(t *testing.T) {
	dir := metricGitInit(t, map[string]string{
		"frontend/dash.ts": `const a = "success_rate"; const b = "success_rate";`,
	})
	base := &graph.Graph{Nodes: []graph.Node{
		{ID: "metric:success-rate", Kind: graph.NodeMetric, Label: "success_rate"},
	}}
	current := graph.Graph{}
	r := computeOrphanedMetricAliases(dir, base, current)
	if r.Score != 1 {
		t.Errorf("expected deduped to 1, got %d", r.Score)
	}
}

func TestVerdictTelemetryOnOrphanedMetrics(t *testing.T) {
	r := Report{OrphanedMetricAliases: OrphanedMetricAliases{
		Score:   1,
		Orphans: []OrphanedMetricAlias{{File: "f.ts", OrphanName: "x"}},
	}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}
