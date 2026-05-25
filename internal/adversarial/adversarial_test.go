package adversarial

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fireharp/coherence/internal/graph"
)

func TestLoadManifestDefaultsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.yml")
	if err := os.WriteFile(path, []byte(`version: 1
repos:
  - id: self
    path: .
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("repos=%d, want 1", len(m.Repos))
	}
	r := m.Repos[0]
	if r.Path != dir {
		t.Fatalf("path=%q, want %q", r.Path, dir)
	}
	if r.Weight != 1 {
		t.Fatalf("weight=%d, want default 1", r.Weight)
	}
	if len(r.Include) != 1 || r.Include[0] != "**" {
		t.Fatalf("include default = %v", r.Include)
	}
	if len(r.Exclude) == 0 {
		t.Fatal("exclude defaults should be populated")
	}
}

func TestLoadManifestFilePathResolvesFromBaseDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "corpus.yml"), []byte(`version: 1
repos:
  - id: self
    path: .
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest("corpus.yml", dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Repos[0].Path != dir {
		t.Fatalf("repo path=%q, want %q", m.Repos[0].Path, dir)
	}
}

func TestLoadManifestRejectsRemotePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.yml")
	if err := os.WriteFile(path, []byte(`version: 1
repos:
  - id: remote
    path: https://github.com/example/repo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path, dir)
	if err == nil {
		t.Fatal("expected remote path validation error")
	}
}

func TestLoadTaxonomyPathResolvesFromBaseDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "taxonomy.yml"), []byte(`version: 1
mutations:
  - id: local-taxonomy
    operation: append_text
    target_kinds: [file]
    expected_meters: [unknown_id_references]
    selector:
      path_glob: AGENTS.md
    edit:
      text: "\nUS-999\n"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	specs, err := LoadTaxonomy("taxonomy.yml", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "local-taxonomy" {
		t.Fatalf("specs=%+v", specs)
	}
}

func TestLoadTaxonomyRejectsEmptyMutations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "taxonomy.yml"), []byte("version: 1\nmutations: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTaxonomy("taxonomy.yml", dir)
	if err == nil || !strings.Contains(err.Error(), "mutations must not be empty") {
		t.Fatalf("LoadTaxonomy err=%v, want empty mutations error", err)
	}
}

func TestValidateSpecsRejectsBadOperation(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad",
		Operation:      "explode",
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
	}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuiltinFirstTwentyCoverDeterministicSpecs(t *testing.T) {
	specs := BuiltinSpecs()
	if len(specs) < 21 {
		t.Fatalf("builtin specs=%d, want at least 21", len(specs))
	}
	for i, spec := range specs[:20] {
		if spec.RequiresLLM {
			t.Fatalf("spec %d (%s) requires LLM; first 20 should be deterministic", i, spec.ID)
		}
	}
	if specs[19].ID != "ADV-021-broken-implements-chain" {
		t.Fatalf("20th spec=%s, want ADV-021-broken-implements-chain", specs[19].ID)
	}
	if specs[20].ID != "ADV-020-llm-contradiction" {
		t.Fatalf("21st spec=%s, want ADV-020-llm-contradiction", specs[20].ID)
	}
}

func TestValidateSpecsRejectsUnknownMeterAndEdge(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-meter",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"not_a_meter"},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected unknown meter validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-edge",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		Selector:       Selector{HasIncomingEdge: "not_an_edge"},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected unknown edge validation error")
	}
}

func TestValidateSpecsRejectsBadSkipConditions(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-env",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireEnv: []string{""}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad env skip condition validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-file",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireFiles: []string{"../outside"}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad file skip condition validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-engine",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		SkipConditions: SkipConditions{RequireOptionalEngines: []string{"not_an_engine"}},
		Edit:           Edit{Text: "x"},
	}})
	if err == nil {
		t.Fatal("expected bad optional engine skip condition validation error")
	}
}

func TestValidateSpecsRejectsUnsafeEditPaths(t *testing.T) {
	err := validateSpecs([]Spec{{
		ID:             "bad-edit-path",
		Operation:      opAddFile,
		TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
		ExpectedMeters: []string{"broken_links"},
		Edit:           Edit{Path: "../outside.md", Content: "x"},
	}})
	if err == nil {
		t.Fatal("expected unsafe edit path validation error")
	}
	err = validateSpecs([]Spec{{
		ID:             "bad-rendered-edit-path",
		Operation:      opRenameFile,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"broken_links"},
		Edit:           Edit{NewPath: "${target_dir}/../../outside.md"},
	}})
	if err == nil {
		t.Fatal("expected unsafe templated edit path validation error")
	}
	for _, p := range []string{".git/config", ".coherence/drift.json"} {
		err = validateSpecs([]Spec{{
			ID:             "bad-reserved-" + strings.ReplaceAll(p, "/", "-"),
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters: []string{"broken_links"},
			Edit:           Edit{Path: p, Content: "x"},
		}})
		if err == nil {
			t.Fatalf("expected reserved edit path validation error for %s", p)
		}
	}
}

func TestSelectTargetUsesGraphAndSelector(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "file:a.go", Kind: graph.NodeFile, Path: "a.go"},
			{ID: "file:b.go", Kind: graph.NodeFile, Path: "b.go"},
			{ID: "test:b_test.go", Kind: graph.NodeTest, Path: "b_test.go"},
		},
		Edges: []graph.Edge{{From: "test:b_test.go", To: "file:b.go", Kind: graph.EdgeVerifies}},
	}
	target, ok := selectTarget(g, Spec{
		TargetKinds: []graph.NodeKind{graph.NodeFile},
		Selector:    Selector{HasIncomingEdge: string(graph.EdgeVerifies)},
	}, randForTest())
	if !ok {
		t.Fatal("expected target")
	}
	if target.ID != "file:b.go" {
		t.Fatalf("target=%s, want file:b.go", target.ID)
	}
}

func TestApplyMutationReplaceText(t *testing.T) {
	dir := t.TempDir()
	if err := writeFiles(dir, map[string]string{"a.txt": "hello old\n"}); err != nil {
		t.Fatal(err)
	}
	err := applyMutation(dir, Spec{
		Operation: opReplaceText,
		Edit:      Edit{Old: "old", New: "new"},
	}, Target{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello new\n" {
		t.Fatalf("content=%q", string(data))
	}
}

func TestApplyMutationRejectsUnsafeRenderedPath(t *testing.T) {
	dir := t.TempDir()
	err := applyMutation(dir, Spec{
		Operation: opAddFile,
		Edit:      Edit{Path: "${target_dir}/../../outside.md", Content: "x"},
	}, Target{Path: "docs/a.md"})
	if err == nil {
		t.Fatal("expected unsafe rendered path error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "..", "outside.md")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file stat err=%v, want not exists", statErr)
	}
}

func TestClassifyAllowsMovementMeters(t *testing.T) {
	res := Result{
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"broken_links", "semantic_movement", "neighborhood_drift"},
	}
	classify(&res, Spec{ExpectedMeters: []string{"broken_links"}})
	if res.Classification != ClassificationHit {
		t.Fatalf("classification=%s, want hit: %+v", res.Classification, res)
	}
}

func TestClassifyUsesRequestedVocabularyWhenBothMissAndFP(t *testing.T) {
	res := Result{
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"stale_tests"},
	}
	classify(&res, Spec{ExpectedMeters: []string{"broken_links"}})
	if res.Classification != ClassificationMiss {
		t.Fatalf("classification=%s, want %s", res.Classification, ClassificationMiss)
	}
	if len(res.FalseNegatives) != 1 || len(res.FalsePositives) != 1 {
		t.Fatalf("expected both fn and fp details: %+v", res)
	}
}

func TestClusterKeyStable(t *testing.T) {
	r := Result{
		MutationID:     "m",
		ExpectedMeters: []string{"a"},
		ActualMeters:   []string{"b"},
		TargetNode:     Target{Kind: graph.NodeDoc, Path: "docs/a.md"},
		Error:          "boom: detail",
	}
	s := Spec{Operation: opRemoveFile}
	if clusterKey(r, s) != clusterKey(r, s) {
		t.Fatal("cluster key should be stable")
	}
}

func TestErroredClusterKeyKeepsMutationOperation(t *testing.T) {
	base := Result{MutationID: "mut", ExpectedMeters: []string{"broken_links"}}
	remove := errored(base, Spec{ID: "mut", Operation: opRemoveFile, ExpectedMeters: []string{"broken_links"}}, time.Now(), fmt.Errorf("boom: detail"))
	append := errored(base, Spec{ID: "mut", Operation: opAppendText, ExpectedMeters: []string{"broken_links"}}, time.Now(), fmt.Errorf("boom: detail"))
	if remove.ClusterKey == "" || append.ClusterKey == "" {
		t.Fatalf("missing cluster keys: remove=%q append=%q", remove.ClusterKey, append.ClusterKey)
	}
	if remove.ClusterKey == append.ClusterKey {
		t.Fatalf("errored cluster keys should differ by operation: %q", remove.ClusterKey)
	}
}

func TestBuildRefinementsFromMissCluster(t *testing.T) {
	results := []Result{{
		MutationID:     "mut",
		Hypothesis:     "mut should activate trace_coverage",
		ExpectedMeters: []string{"trace_coverage"},
		ActualMeters:   []string{"semantic_movement"},
		Classification: ClassificationMiss,
		FalseNegatives: []string{"trace_coverage"},
		ClusterKey:     "abc",
	}}
	clusters := clusterResults(results)
	refs := buildRefinements(results, clusters)
	if len(refs) != 1 {
		t.Fatalf("refinements=%d, want 1", len(refs))
	}
	if refs[0].NextExperiment == "" || refs[0].SuggestedAction == "" {
		t.Fatalf("refinement missing guidance: %+v", refs[0])
	}
}

func TestSummaryIncludesGroupedRates(t *testing.T) {
	summary := summarize([]Result{
		{MutationID: "a", ExpectedMeters: []string{"broken_links"}, Classification: ClassificationHit},
		{MutationID: "a", ExpectedMeters: []string{"broken_links"}, Classification: ClassificationMiss, FalseNegatives: []string{"broken_links"}},
		{MutationID: "b", ExpectedMeters: []string{"stale_tests"}, Classification: ClassificationFP, FalsePositives: []string{"stale_tests"}},
	})
	links := summary.ByExpectedMeter["broken_links"]
	if links.HitRate != 0.5 || links.FalseNegativeRate != 0.5 {
		t.Fatalf("broken_links stats=%+v, want hit/fn rates 0.5", links)
	}
	mut := summary.ByMutation["b"]
	if mut.FalsePositiveRate != 1 {
		t.Fatalf("mutation b stats=%+v, want fp rate 1", mut)
	}
	fpMeter := summary.ByMeter["stale_tests"]
	if fpMeter.Total != 1 || fpMeter.FalsePositiveRate != 1 {
		t.Fatalf("by-meter stale_tests stats=%+v, want one false positive", fpMeter)
	}
}

func TestBuildRefinementsFromAllHitsContinuesLoop(t *testing.T) {
	results := []Result{{
		MutationID:     "mut",
		Hypothesis:     "mut should activate broken_links",
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"broken_links"},
		Classification: ClassificationHit,
	}}
	refs := buildRefinements(results, clusterResults(results))
	if len(refs) != 1 {
		t.Fatalf("refinements=%d, want 1 continuation", len(refs))
	}
	if refs[0].NextExperiment == "" {
		t.Fatalf("missing next experiment: %+v", refs[0])
	}
}

func TestReorderSpecsForRefinementPrioritizesClusterMisses(t *testing.T) {
	specs := []Spec{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	prev := Report{Clusters: []Cluster{{MutationIDs: []string{"c"}}}}
	got := reorderSpecsForRefinement(specs, prev)
	if got[0].ID != "c" {
		t.Fatalf("first spec=%s, want c", got[0].ID)
	}
}

func TestReorderSpecsForCleanRunRotates(t *testing.T) {
	specs := []Spec{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	prev := Report{Iterations: 1}
	got := reorderSpecsForRefinement(specs, prev)
	if got[0].ID != "b" {
		t.Fatalf("first spec=%s, want b", got[0].ID)
	}
}

func TestRunEmbeddedAdversarialNoLLMKey(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: len(BuiltinSpecs()), Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if report.Pass {
		t.Fatalf("expected exploration demo to keep the run non-passing: %+v", report.Summary)
	}
	if report.Summary.Errored != 0 || report.Summary.FalseNegatives != 35 || report.Summary.FalsePositives != 0 {
		t.Fatalf("unexpected failures: %+v", report.Summary)
	}
	if report.Summary.Skipped != 1 {
		t.Fatalf("skipped=%d, want 1 LLM skip", report.Summary.Skipped)
	}
	demo := findResult(report.Results, "ADV-022-agent-skill-unknown-id-demo")
	if demo == nil {
		t.Fatal("missing ADV-022 exploration demo result")
	}
	if demo.Classification != ClassificationMiss ||
		len(demo.FalseNegatives) != 1 ||
		demo.FalseNegatives[0] != "unknown_id_references" ||
		len(demo.FalsePositives) != 0 {
		t.Fatalf("demo result=%+v, want false negative for unknown_id_references", *demo)
	}
	splitMetric := findResult(report.Results, "ADV-023-split-string-metric-alias-demo")
	if splitMetric == nil {
		t.Fatal("missing ADV-023 exploration demo result")
	}
	if splitMetric.Classification != ClassificationMiss ||
		len(splitMetric.FalseNegatives) != 1 ||
		splitMetric.FalseNegatives[0] != "orphaned_metric_aliases" ||
		len(splitMetric.FalsePositives) != 0 {
		t.Fatalf("split metric result=%+v, want false negative for orphaned_metric_aliases", *splitMetric)
	}
	dynamicEndpoint := findResult(report.Results, "ADV-024-dynamic-ts-endpoint-demo")
	if dynamicEndpoint == nil {
		t.Fatal("missing ADV-024 exploration demo result")
	}
	if dynamicEndpoint.Classification != ClassificationMiss ||
		len(dynamicEndpoint.FalseNegatives) != 1 ||
		dynamicEndpoint.FalseNegatives[0] != "orphan_endpoints" ||
		len(dynamicEndpoint.FalsePositives) != 0 {
		t.Fatalf("dynamic endpoint result=%+v, want false negative for orphan_endpoints", *dynamicEndpoint)
	}
	pythonDynamic := findResult(report.Results, "ADV-025-python-dynamic-import-demo")
	if pythonDynamic == nil {
		t.Fatal("missing ADV-025 exploration demo result")
	}
	if pythonDynamic.Classification != ClassificationMiss ||
		len(pythonDynamic.FalseNegatives) != 1 ||
		pythonDynamic.FalseNegatives[0] != "dangling_imports" ||
		len(pythonDynamic.FalsePositives) != 0 {
		t.Fatalf("python dynamic import result=%+v, want false negative for dangling_imports", *pythonDynamic)
	}
	rawADR := findResult(report.Results, "ADV-026-raw-adr-citation-demo")
	if rawADR == nil {
		t.Fatal("missing ADV-026 exploration demo result")
	}
	if rawADR.Classification != ClassificationMiss ||
		len(rawADR.FalseNegatives) != 1 ||
		rawADR.FalseNegatives[0] != "stale_decision_links" ||
		len(rawADR.FalsePositives) != 0 {
		t.Fatalf("raw ADR result=%+v, want false negative for stale_decision_links", *rawADR)
	}
	refStyle := findResult(report.Results, "ADV-027-reference-style-link-demo")
	if refStyle == nil {
		t.Fatal("missing ADV-027 exploration demo result")
	}
	if refStyle.Classification != ClassificationMiss ||
		len(refStyle.FalseNegatives) != 1 ||
		refStyle.FalseNegatives[0] != "broken_links" ||
		len(refStyle.FalsePositives) != 0 {
		t.Fatalf("reference-style link result=%+v, want false negative for broken_links", *refStyle)
	}
	reexport := findResult(report.Results, "ADV-028-ts-reexport-dangling-import-demo")
	if reexport == nil {
		t.Fatal("missing ADV-028 exploration demo result")
	}
	if reexport.Classification != ClassificationMiss ||
		len(reexport.FalseNegatives) != 1 ||
		reexport.FalseNegatives[0] != "dangling_imports" ||
		len(reexport.FalsePositives) != 0 {
		t.Fatalf("TS re-export result=%+v, want false negative for dangling_imports", *reexport)
	}
	htmlLink := findResult(report.Results, "ADV-029-html-markdown-link-demo")
	if htmlLink == nil {
		t.Fatal("missing ADV-029 exploration demo result")
	}
	if htmlLink.Classification != ClassificationMiss ||
		len(htmlLink.FalseNegatives) != 1 ||
		htmlLink.FalseNegatives[0] != "broken_links" ||
		len(htmlLink.FalsePositives) != 0 {
		t.Fatalf("HTML link result=%+v, want false negative for broken_links", *htmlLink)
	}
	pythonEndpoint := findResult(report.Results, "ADV-030-python-dynamic-endpoint-demo")
	if pythonEndpoint == nil {
		t.Fatal("missing ADV-030 exploration demo result")
	}
	if pythonEndpoint.Classification != ClassificationMiss ||
		len(pythonEndpoint.FalseNegatives) != 1 ||
		pythonEndpoint.FalseNegatives[0] != "orphan_endpoints" ||
		len(pythonEndpoint.FalsePositives) != 0 {
		t.Fatalf("python dynamic endpoint result=%+v, want false negative for orphan_endpoints", *pythonEndpoint)
	}
	tsDynamicImport := findResult(report.Results, "ADV-031-ts-dynamic-import-demo")
	if tsDynamicImport == nil {
		t.Fatal("missing ADV-031 exploration demo result")
	}
	if tsDynamicImport.Classification != ClassificationMiss ||
		len(tsDynamicImport.FalseNegatives) != 1 ||
		tsDynamicImport.FalseNegatives[0] != "dangling_imports" ||
		len(tsDynamicImport.FalsePositives) != 0 {
		t.Fatalf("TS dynamic import result=%+v, want false negative for dangling_imports", *tsDynamicImport)
	}
	goEndpoint := findResult(report.Results, "ADV-032-go-dynamic-endpoint-demo")
	if goEndpoint == nil {
		t.Fatal("missing ADV-032 exploration demo result")
	}
	if goEndpoint.Classification != ClassificationMiss ||
		len(goEndpoint.FalseNegatives) != 1 ||
		goEndpoint.FalseNegatives[0] != "orphan_endpoints" ||
		len(goEndpoint.FalsePositives) != 0 {
		t.Fatalf("Go dynamic endpoint result=%+v, want false negative for orphan_endpoints", *goEndpoint)
	}
	pythonAbsolute := findResult(report.Results, "ADV-033-python-absolute-import-demo")
	if pythonAbsolute == nil {
		t.Fatal("missing ADV-033 exploration demo result")
	}
	if pythonAbsolute.Classification != ClassificationMiss ||
		len(pythonAbsolute.FalseNegatives) != 1 ||
		pythonAbsolute.FalseNegatives[0] != "dangling_imports" ||
		len(pythonAbsolute.FalsePositives) != 0 {
		t.Fatalf("python absolute import result=%+v, want false negative for dangling_imports", *pythonAbsolute)
	}
	tsAlias := findResult(report.Results, "ADV-034-ts-path-alias-import-demo")
	if tsAlias == nil {
		t.Fatal("missing ADV-034 exploration demo result")
	}
	if tsAlias.Classification != ClassificationMiss ||
		len(tsAlias.FalseNegatives) != 1 ||
		tsAlias.FalseNegatives[0] != "dangling_imports" ||
		len(tsAlias.FalsePositives) != 0 {
		t.Fatalf("TS path alias import result=%+v, want false negative for dangling_imports", *tsAlias)
	}
	pythonImport := findResult(report.Results, "ADV-035-python-import-statement-demo")
	if pythonImport == nil {
		t.Fatal("missing ADV-035 exploration demo result")
	}
	if pythonImport.Classification != ClassificationMiss ||
		len(pythonImport.FalseNegatives) != 1 ||
		pythonImport.FalseNegatives[0] != "dangling_imports" ||
		len(pythonImport.FalsePositives) != 0 {
		t.Fatalf("python import statement result=%+v, want false negative for dangling_imports", *pythonImport)
	}
	refADR := findResult(report.Results, "ADV-036-reference-style-adr-citation-demo")
	if refADR == nil {
		t.Fatal("missing ADV-036 exploration demo result")
	}
	if refADR.Classification != ClassificationMiss ||
		len(refADR.FalseNegatives) != 1 ||
		refADR.FalseNegatives[0] != "stale_decision_links" ||
		len(refADR.FalsePositives) != 0 {
		t.Fatalf("reference-style ADR result=%+v, want false negative for stale_decision_links", *refADR)
	}
	mdxStory := findResult(report.Results, "ADV-037-mdx-user-story-demo")
	if mdxStory == nil {
		t.Fatal("missing ADV-037 exploration demo result")
	}
	if mdxStory.Classification != ClassificationMiss ||
		len(mdxStory.FalseNegatives) != 1 ||
		mdxStory.FalseNegatives[0] != "unimplemented_stories" ||
		len(mdxStory.FalsePositives) != 0 {
		t.Fatalf("MDX story result=%+v, want false negative for unimplemented_stories", *mdxStory)
	}
	measureMetric := findResult(report.Results, "ADV-038-metric-measure-name-demo")
	if measureMetric == nil {
		t.Fatal("missing ADV-038 exploration demo result")
	}
	if measureMetric.Classification != ClassificationMiss ||
		len(measureMetric.FalseNegatives) != 1 ||
		measureMetric.FalseNegatives[0] != "orphaned_metric_aliases" ||
		len(measureMetric.FalsePositives) != 0 {
		t.Fatalf("metric measure result=%+v, want false negative for orphaned_metric_aliases", *measureMetric)
	}
	tsTestsDir := findResult(report.Results, "ADV-039-ts-tests-dir-stale-test-demo")
	if tsTestsDir == nil {
		t.Fatal("missing ADV-039 exploration demo result")
	}
	if tsTestsDir.Classification != ClassificationMiss ||
		len(tsTestsDir.FalseNegatives) != 1 ||
		tsTestsDir.FalseNegatives[0] != "stale_tests" ||
		len(tsTestsDir.FalsePositives) != 0 {
		t.Fatalf("TS __tests__ stale test result=%+v, want false negative for stale_tests", *tsTestsDir)
	}
	mdxLink := findResult(report.Results, "ADV-040-mdx-broken-link-demo")
	if mdxLink == nil {
		t.Fatal("missing ADV-040 exploration demo result")
	}
	if mdxLink.Classification != ClassificationMiss ||
		len(mdxLink.FalseNegatives) != 1 ||
		mdxLink.FalseNegatives[0] != "broken_links" ||
		len(mdxLink.FalsePositives) != 0 {
		t.Fatalf("MDX broken link result=%+v, want false negative for broken_links", *mdxLink)
	}
	tsRequire := findResult(report.Results, "ADV-041-ts-require-dangling-import-demo")
	if tsRequire == nil {
		t.Fatal("missing ADV-041 exploration demo result")
	}
	if tsRequire.Classification != ClassificationMiss ||
		len(tsRequire.FalseNegatives) != 1 ||
		tsRequire.FalseNegatives[0] != "dangling_imports" ||
		len(tsRequire.FalsePositives) != 0 {
		t.Fatalf("TS require result=%+v, want false negative for dangling_imports", *tsRequire)
	}
	tsReference := findResult(report.Results, "ADV-042-ts-triple-slash-reference-demo")
	if tsReference == nil {
		t.Fatal("missing ADV-042 exploration demo result")
	}
	if tsReference.Classification != ClassificationMiss ||
		len(tsReference.FalseNegatives) != 1 ||
		tsReference.FalseNegatives[0] != "dangling_imports" ||
		len(tsReference.FalsePositives) != 0 {
		t.Fatalf("TS triple-slash reference result=%+v, want false negative for dangling_imports", *tsReference)
	}
	pythonTestsDir := findResult(report.Results, "ADV-043-python-tests-dir-stale-test-demo")
	if pythonTestsDir == nil {
		t.Fatal("missing ADV-043 exploration demo result")
	}
	if pythonTestsDir.Classification != ClassificationMiss ||
		len(pythonTestsDir.FalseNegatives) != 1 ||
		pythonTestsDir.FalseNegatives[0] != "stale_tests" ||
		len(pythonTestsDir.FalsePositives) != 0 {
		t.Fatalf("Python tests/ stale test result=%+v, want false negative for stale_tests", *pythonTestsDir)
	}
	pythonDotted := findResult(report.Results, "ADV-044-python-dotted-route-demo")
	if pythonDotted == nil {
		t.Fatal("missing ADV-044 exploration demo result")
	}
	if pythonDotted.Classification != ClassificationMiss ||
		len(pythonDotted.FalseNegatives) != 1 ||
		pythonDotted.FalseNegatives[0] != "orphan_endpoints" ||
		len(pythonDotted.FalsePositives) != 0 {
		t.Fatalf("Python dotted endpoint result=%+v, want false negative for orphan_endpoints", *pythonDotted)
	}
	wikiLink := findResult(report.Results, "ADV-045-markdown-wiki-link-demo")
	if wikiLink == nil {
		t.Fatal("missing ADV-045 exploration demo result")
	}
	if wikiLink.Classification != ClassificationMiss ||
		len(wikiLink.FalseNegatives) != 1 ||
		wikiLink.FalseNegatives[0] != "broken_links" ||
		len(wikiLink.FalsePositives) != 0 {
		t.Fatalf("Markdown wiki-link result=%+v, want false negative for broken_links", *wikiLink)
	}
	importEquals := findResult(report.Results, "ADV-046-ts-import-equals-require-demo")
	if importEquals == nil {
		t.Fatal("missing ADV-046 exploration demo result")
	}
	if importEquals.Classification != ClassificationMiss ||
		len(importEquals.FalseNegatives) != 1 ||
		importEquals.FalseNegatives[0] != "dangling_imports" ||
		len(importEquals.FalsePositives) != 0 {
		t.Fatalf("TS import-equals result=%+v, want false negative for dangling_imports", *importEquals)
	}
	multilineImport := findResult(report.Results, "ADV-047-ts-multiline-import-demo")
	if multilineImport == nil {
		t.Fatal("missing ADV-047 exploration demo result")
	}
	if multilineImport.Classification != ClassificationMiss ||
		len(multilineImport.FalseNegatives) != 1 ||
		multilineImport.FalseNegatives[0] != "dangling_imports" ||
		len(multilineImport.FalsePositives) != 0 {
		t.Fatalf("TS multiline import result=%+v, want false negative for dangling_imports", *multilineImport)
	}
	agentDoc := findResult(report.Results, "ADV-048-agent-doc-unknown-id-demo")
	if agentDoc == nil {
		t.Fatal("missing ADV-048 exploration demo result")
	}
	if agentDoc.Classification != ClassificationMiss ||
		len(agentDoc.FalseNegatives) != 1 ||
		agentDoc.FalseNegatives[0] != "unknown_id_references" ||
		len(agentDoc.FalsePositives) != 0 {
		t.Fatalf("agent doc unknown id result=%+v, want false negative for unknown_id_references", *agentDoc)
	}
	markdownExt := findResult(report.Results, "ADV-049-markdown-extension-link-demo")
	if markdownExt == nil {
		t.Fatal("missing ADV-049 exploration demo result")
	}
	if markdownExt.Classification != ClassificationMiss ||
		len(markdownExt.FalseNegatives) != 1 ||
		markdownExt.FalseNegatives[0] != "broken_links" ||
		len(markdownExt.FalsePositives) != 0 {
		t.Fatalf(".markdown broken link result=%+v, want false negative for broken_links", *markdownExt)
	}
	vueMetric := findResult(report.Results, "ADV-050-vue-metric-alias-demo")
	if vueMetric == nil {
		t.Fatal("missing ADV-050 exploration demo result")
	}
	if vueMetric.Classification != ClassificationMiss ||
		len(vueMetric.FalseNegatives) != 1 ||
		vueMetric.FalseNegatives[0] != "orphaned_metric_aliases" ||
		len(vueMetric.FalsePositives) != 0 {
		t.Fatalf("Vue metric alias result=%+v, want false negative for orphaned_metric_aliases", *vueMetric)
	}
	cssImport := findResult(report.Results, "ADV-051-css-import-demo")
	if cssImport == nil {
		t.Fatal("missing ADV-051 exploration demo result")
	}
	if cssImport.Classification != ClassificationMiss ||
		len(cssImport.FalseNegatives) != 1 ||
		cssImport.FalseNegatives[0] != "dangling_imports" ||
		len(cssImport.FalsePositives) != 0 {
		t.Fatalf("CSS import result=%+v, want false negative for dangling_imports", *cssImport)
	}
	manualRoute := findResult(report.Results, "ADV-052-fastapi-add-api-route-demo")
	if manualRoute == nil {
		t.Fatal("missing ADV-052 exploration demo result")
	}
	if manualRoute.Classification != ClassificationMiss ||
		len(manualRoute.FalseNegatives) != 1 ||
		manualRoute.FalseNegatives[0] != "orphan_endpoints" ||
		len(manualRoute.FalsePositives) != 0 {
		t.Fatalf("FastAPI manual route result=%+v, want false negative for orphan_endpoints", *manualRoute)
	}
	quotedID := findResult(report.Results, "ADV-053-quoted-code-typed-id-demo")
	if quotedID == nil {
		t.Fatal("missing ADV-053 exploration demo result")
	}
	if quotedID.Classification != ClassificationMiss ||
		len(quotedID.FalseNegatives) != 1 ||
		quotedID.FalseNegatives[0] != "unknown_id_references" ||
		len(quotedID.FalsePositives) != 0 {
		t.Fatalf("quoted code ID result=%+v, want false negative for unknown_id_references", *quotedID)
	}
	mdxMetric := findResult(report.Results, "ADV-054-mdx-metric-prop-demo")
	if mdxMetric == nil {
		t.Fatal("missing ADV-054 exploration demo result")
	}
	if mdxMetric.Classification != ClassificationMiss ||
		len(mdxMetric.FalseNegatives) != 1 ||
		mdxMetric.FalseNegatives[0] != "orphaned_metric_aliases" ||
		len(mdxMetric.FalsePositives) != 0 {
		t.Fatalf("MDX metric prop result=%+v, want false negative for orphaned_metric_aliases", *mdxMetric)
	}
	goImport := findResult(report.Results, "ADV-055-go-dangling-import-demo")
	if goImport == nil {
		t.Fatal("missing ADV-055 exploration demo result")
	}
	if goImport.Classification != ClassificationMiss ||
		len(goImport.FalseNegatives) != 1 ||
		goImport.FalseNegatives[0] != "dangling_imports" ||
		len(goImport.FalsePositives) != 0 {
		t.Fatalf("Go dangling import result=%+v, want false negative for dangling_imports", *goImport)
	}
	angleLink := findResult(report.Results, "ADV-056-markdown-angle-autolink-demo")
	if angleLink == nil {
		t.Fatal("missing ADV-056 exploration demo result")
	}
	if angleLink.Classification != ClassificationMiss ||
		len(angleLink.FalseNegatives) != 1 ||
		angleLink.FalseNegatives[0] != "broken_links" ||
		len(angleLink.FalsePositives) != 0 {
		t.Fatalf("Markdown angle autolink result=%+v, want false negative for broken_links", *angleLink)
	}
}

func TestRunFirstTwentyHitsEveryDeterministicMeter(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: 20, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pass || report.Summary.Skipped != 0 || report.Summary.Errored != 0 {
		t.Fatalf("expected clean deterministic run: %+v", report.Summary)
	}
	expectedMeters := map[string]bool{}
	for _, spec := range BuiltinSpecs()[:20] {
		if spec.RequiresLLM {
			t.Fatalf("deterministic window includes llm spec %s", spec.ID)
		}
		for _, meter := range spec.ExpectedMeters {
			expectedMeters[meter] = true
		}
	}
	for meter := range expectedMeters {
		stats, ok := report.Summary.ByExpectedMeter[meter]
		if !ok {
			t.Fatalf("missing expected meter %s in summary", meter)
		}
		if stats.Total == 0 || stats.Hits != stats.Total || stats.FalseNegatives != 0 || stats.FalsePositives != 0 {
			t.Fatalf("meter %s stats=%+v, want all hits", meter, stats)
		}
	}
	var noop *Result
	for i := range report.Results {
		if report.Results[i].MutationID == "ADV-014-semantic-noop-typo" {
			noop = &report.Results[i]
			break
		}
	}
	if noop == nil {
		t.Fatal("missing semantic noop result")
	}
	if noop.Classification != ClassificationHit || len(noop.ActualMeters) != 0 {
		t.Fatalf("semantic noop result=%+v, want hit with no active meters", *noop)
	}
}

func TestRunSkipsOnExplicitConditions(t *testing.T) {
	t.Setenv("ADV_TEST_REQUIRED_ENV", "")
	envSpec := Spec{
		ID:             "skip-env",
		Operation:      opAppendText,
		TargetKinds:    []graph.NodeKind{graph.NodeFile},
		ExpectedMeters: []string{"unknown_id_references"},
		Selector:       Selector{PathGlob: "AGENTS.md"},
		SkipConditions: SkipConditions{RequireEnv: []string{"ADV_TEST_REQUIRED_ENV"}},
		Edit:           Edit{Text: "\nUS-999\n"},
	}
	res := runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: envSpec, seed: 1})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "ADV_TEST_REQUIRED_ENV") {
		t.Fatalf("env skip result=%+v", res)
	}
	fileSpec := envSpec
	fileSpec.ID = "skip-file"
	fileSpec.SkipConditions = SkipConditions{RequireFiles: []string{"missing.txt"}}
	res = runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: fileSpec, seed: 1})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "missing.txt") {
		t.Fatalf("file skip result=%+v", res)
	}

	files := embeddedAgentRepo()
	files["ontology.yml"] = "version: 1\nrules: []\n"
	engineSpec := BuiltinSpecs()[17]
	res = runOne("run", workItem{
		iteration: 1,
		repo:      corpusRepo{RepoEntry: RepoEntry{ID: "optional-disabled"}, Files: files},
		spec:      engineSpec,
		seed:      1,
	})
	if res.Classification != ClassificationSkipped || !strings.Contains(res.SkipReason, "callsite_blast_radius") {
		t.Fatalf("optional-engine skip result=%+v", res)
	}
}

func TestRunParallelMatchesSerial(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	serial, err := Run(Options{RootDir: root, Iterations: 8, Seed: 9, Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	parallel, err := Run(Options{RootDir: root, Iterations: 8, Seed: 9, Jobs: 4})
	if err != nil {
		t.Fatal(err)
	}
	if serial.Summary.Hits != parallel.Summary.Hits ||
		serial.Summary.FalseNegatives != parallel.Summary.FalseNegatives ||
		serial.Summary.FalsePositives != parallel.Summary.FalsePositives ||
		serial.Summary.Skipped != parallel.Summary.Skipped ||
		serial.Summary.Errored != parallel.Summary.Errored {
		t.Fatalf("summary mismatch:\nserial=%+v\nparallel=%+v", serial.Summary, parallel.Summary)
	}
	serialSig := resultSignatures(serial.Results)
	parallelSig := resultSignatures(parallel.Results)
	if strings.Join(serialSig, "\n") != strings.Join(parallelSig, "\n") {
		t.Fatalf("result signatures differ:\nserial=%v\nparallel=%v", serialSig, parallelSig)
	}
}

func TestRunManifestRepoDoesNotMutateSource(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, embeddedAgentRepo()); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(source, "pkg", "policy", "policy.go")
	before, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "corpus.yml")
	if err := os.WriteFile(manifestPath, []byte("version: 1\nrepos:\n  - id: local\n    path: source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Run(Options{RootDir: root, ManifestPath: manifestPath, Iterations: 1, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].MutationID != "ADV-001-stale-go-test" {
		t.Fatalf("unexpected result: %+v", report.Results)
	}
	after, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("source repo file changed during adversarial run")
	}
	status, err := runGit(source, "status", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("source repo dirty after adversarial run:\n%s", status)
	}
}

func TestMaterializeErrorsWhenFiltersSelectNoFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	_, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "empty-selection",
		Path:    source,
		Include: []string{"docs/**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err == nil || !strings.Contains(err.Error(), "selected no tracked files") {
		t.Fatalf("materialize err=%v, want selected no tracked files", err)
	}
}

func TestMaterializePreservesTrackedGitignoreAndIgnoresRuntimeState(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		".gitignore":   "dist/\n",
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
		"README.md":    "# Fixture\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}

	dir, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "dist/\n" {
		t.Fatalf("materialized .gitignore=%q, want tracked source content", string(data))
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".coherence/") {
		t.Fatalf("runtime exclude missing .coherence/: %q", string(exclude))
	}
	status, err := runGit(dir, "status", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("materialized repo dirty after baseline graph writes:\n%s", status)
	}
}

func TestMaterializePreservesSafeSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":      "# Agent Notes\n",
		"ontology.yml":   "version: 1\nrules: []\n",
		"docs/target.md": "# Target\n",
		"docs/README.md": "# Docs\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.md", filepath.Join(source, "docs", "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	dir, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target, err := os.Readlink(filepath.Join(dir, "docs", "link.md"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "target.md" {
		t.Fatalf("materialized symlink target=%q, want target.md", target)
	}
}

func TestMaterializeRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.md", filepath.Join(source, "leak.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	_, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err == nil || !strings.Contains(err.Error(), "unsafe symlink") {
		t.Fatalf("materialize err=%v, want unsafe symlink error", err)
	}
}

func TestRequiresLLMSkipsWhenFlagDisabled(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	spec, ok := firstLLMSpec()
	if !ok {
		t.Fatal("missing LLM spec")
	}
	res := runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: spec, seed: 1})
	if res.Classification != ClassificationSkipped {
		t.Fatalf("classification=%s, want skipped", res.Classification)
	}
	if res.SkipReason != "requires --llm" {
		t.Fatalf("skip reason=%q, want requires --llm", res.SkipReason)
	}
}

func TestRunRefineFromAdvancesSeedAndLoadsReport(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	prev, err := Run(Options{RootDir: root, Iterations: 2, Seed: 11, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	next, err := Run(Options{RootDir: root, Iterations: 1, RefineFrom: prev.ReportDir, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	if next.Seed != 12 {
		t.Fatalf("seed=%d, want 12", next.Seed)
	}
	if next.RefineFrom != prev.ReportDir {
		t.Fatalf("refine_from=%q, want %q", next.RefineFrom, prev.ReportDir)
	}
	if next.NextCommand == "" {
		t.Fatal("expected next command")
	}
	if !strings.Contains(next.NextCommand, next.ReportDir) {
		t.Fatalf("next command %q should reference report dir %q", next.NextCommand, next.ReportDir)
	}
}

func TestRunRefineFromRelativePathResolvesFromRoot(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	t.Chdir(t.TempDir())
	prev, err := Run(Options{RootDir: root, Iterations: 2, Seed: 21, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, prev.ReportDir)
	if err != nil {
		t.Fatal(err)
	}
	next, err := Run(Options{RootDir: root, Iterations: 1, RefineFrom: rel})
	if err != nil {
		t.Fatal(err)
	}
	if next.Seed != 22 {
		t.Fatalf("seed=%d, want 22", next.Seed)
	}
	if next.RefineFrom != rel {
		t.Fatalf("refine_from=%q, want raw relative %q", next.RefineFrom, rel)
	}
}

func TestRunCyclesChainsReports(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	loop, err := RunCycles(Options{RootDir: root, Iterations: 2, Cycles: 2, Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(loop.Runs) != 2 {
		t.Fatalf("runs=%d, want 2", len(loop.Runs))
	}
	if !loop.Pass {
		t.Fatalf("expected pass: %+v", loop.Final.Summary)
	}
	first, second := loop.Runs[0], loop.Runs[1]
	if first.ReportDir == "" || second.ReportDir == "" {
		t.Fatalf("cycles should force report writing: first=%q second=%q", first.ReportDir, second.ReportDir)
	}
	if first.ReportDir == second.ReportDir {
		t.Fatalf("cycle report dirs should be unique: %q", first.ReportDir)
	}
	if second.RefineFrom != first.ReportDir {
		t.Fatalf("second refine_from=%q, want %q", second.RefineFrom, first.ReportDir)
	}
	if first.Seed != 5 || second.Seed != 6 {
		t.Fatalf("seeds=(%d,%d), want (5,6)", first.Seed, second.Seed)
	}
	if !strings.Contains(loop.NextCommand, "--cycles=2") {
		t.Fatalf("loop next command should preserve cycle count: %q", loop.NextCommand)
	}
}

func TestRunWriteReportSummaryIncludesExportPath(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	report, err := Run(Options{
		RootDir:      root,
		Iterations:   1,
		Seed:         7,
		WriteReport:  true,
		ExportReport: "docs/adversarial.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReportDir == "" || report.ExportPath == "" {
		t.Fatalf("missing report/export path: report_dir=%q export_path=%q", report.ReportDir, report.ExportPath)
	}
	persisted, err := LoadReport(report.ReportDir)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExportPath != report.ExportPath {
		t.Fatalf("persisted export_path=%q, want %q", persisted.ExportPath, report.ExportPath)
	}
}

func TestParseLLMSpecs(t *testing.T) {
	specs, err := parseLLMSpecs(`{"version":1,"mutations":[{"id":"LLM-1","operation":"append_text","target_kinds":["file"],"expected_meters":["unknown_id_references"],"selector":{"path_glob":"*.go"},"edit":{"text":"US-999"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "LLM-1" {
		t.Fatalf("specs=%+v", specs)
	}
}

func TestLLMSpecPromptOmitsGraphLabels(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "doc:docs/private.md", Kind: graph.NodeDoc, Label: "Private Roadmap Heading", Path: "docs/private.md"},
			{ID: "claim:abc123", Kind: graph.NodeClaim, Label: "Must never send this claim text", Path: "docs/private.md"},
			{ID: "file:docs/private.md", Kind: graph.NodeFile, Label: "private.md", Path: "docs/private.md"},
		},
		Edges: []graph.Edge{{
			From:       "doc:docs/private.md",
			To:         "claim:abc123",
			Kind:       graph.EdgeDefines,
			Provenance: "secret source sentence",
		}},
	}
	prompt := llmSpecPrompt(g, nil)
	for _, leaked := range []string{"Private Roadmap Heading", "Must never send this claim text", "secret source sentence", "label=", "provenance="} {
		if strings.Contains(prompt, leaked) {
			t.Fatalf("prompt leaked %q:\n%s", leaked, prompt)
		}
	}
	if !strings.Contains(prompt, "path=docs/private.md") ||
		!strings.Contains(prompt, "kind=claim") ||
		!strings.Contains(prompt, "kind=defines from=doc:docs/private.md to=claim:abc123") {
		t.Fatalf("prompt missing graph shape:\n%s", prompt)
	}
}

func TestGenerateLLMSpecsValidatesDryRunAndRecords(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	root := t.TempDir()
	body := `{"choices":[{"message":{"role":"assistant","content":"{\"version\":1,\"mutations\":[{\"id\":\"LLM-good\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"unknown_id_references\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"edit\":{\"text\":\"\\nUS-999\\n\"}},{\"id\":\"LLM-bad\",\"operation\":\"replace_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"broken_links\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"edit\":{\"old\":\"not present in file\",\"new\":\"x\"}}]}"}}]}`
	client := fakeHTTPClient{body: body}
	specs, err := GenerateLLMSpecs(Options{
		RootDir:        root,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: client,
	}, builtinCorpus(), BuiltinSpecs())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "LLM-good" {
		t.Fatalf("specs=%+v, want only dry-run-applicable LLM-good", specs)
	}
	files, err := filepath.Glob(filepath.Join(root, ".coherence", "adversarial", "specs", "llm-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("generated spec files=%v, want one", files)
	}
}

func TestGeneratedLLMSpecsHonorSkipConditions(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	root := t.TempDir()
	body := `{"choices":[{"message":{"role":"assistant","content":"{\"version\":1,\"mutations\":[{\"id\":\"LLM-skipped-file\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"unknown_id_references\"],\"selector\":{\"path_glob\":\"AGENTS.md\"},\"skip_conditions\":{\"require_files\":[\"missing.txt\"]},\"edit\":{\"text\":\"\\nUS-999\\n\"}},{\"id\":\"LLM-skipped-engine\",\"operation\":\"append_text\",\"target_kinds\":[\"file\"],\"expected_meters\":[\"dead_code\"],\"selector\":{\"path_glob\":\"pkg/policy/policy.go\"},\"skip_conditions\":{\"require_optional_engines\":[\"dead_code\"]},\"edit\":{\"text\":\"\\nfunc llmUnused() string { return \\\"x\\\" }\\n\"}}]}"}}]}`
	files := embeddedAgentRepo()
	files["ontology.yml"] = "version: 1\nrules: []\n"
	specs, err := GenerateLLMSpecs(Options{
		RootDir:        root,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: fakeHTTPClient{body: body},
	}, []corpusRepo{{RepoEntry: RepoEntry{ID: "optional-disabled"}, Files: files}}, BuiltinSpecs())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Fatalf("specs=%+v, want generated spec filtered by skip condition", specs)
	}
}

func TestWriteGeneratedSpecsDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	specs := []Spec{{ID: "generated", Operation: opAppendText, TargetKinds: []graph.NodeKind{graph.NodeFile}, ExpectedMeters: []string{"unknown_id_references"}}}
	if err := writeGeneratedSpecs(root, specs); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedSpecs(root, specs); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, ".coherence", "adversarial", "specs", "llm-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("generated spec files=%v, want two unique files", files)
	}
}

func TestRunRecordsLLMSpecExpansionStatus(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	report, err := Run(Options{RootDir: t.TempDir(), Iterations: 1, LLMSpecs: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.LLMSpecs.Requested || report.LLMSpecs.Enabled || report.LLMSpecs.Skipped != "missing GROQ_API_KEY" {
		t.Fatalf("unexpected missing-key llm status: %+v", report.LLMSpecs)
	}

	t.Setenv("GROQ_API_KEY", "test-key")
	body := `{"choices":[{"message":{"role":"assistant","content":"not-json"}}]}`
	report, err = Run(Options{
		RootDir:        t.TempDir(),
		Iterations:     1,
		LLMSpecs:       true,
		GroqEndpoint:   "https://groq.test/chat",
		GroqHTTPClient: fakeHTTPClient{body: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.LLMSpecs.Requested || !report.LLMSpecs.Enabled || report.LLMSpecs.Error == "" {
		t.Fatalf("unexpected error llm status: %+v", report.LLMSpecs)
	}
	if !report.Pass {
		t.Fatalf("llm spec expansion error should not fail deterministic run: %+v", report.Summary)
	}
}

func TestWriteReportCreatesArtifacts(t *testing.T) {
	root := t.TempDir()
	report := Report{
		RunID:       "adv-test",
		GeneratedAt: "2026-05-25T00:00:00Z",
		Iterations:  1,
		Summary: Summary{
			Total:   1,
			Hits:    1,
			HitRate: 1,
			ByMeter: map[string]MeterStats{
				"broken_links": {Total: 1, Hits: 1, HitRate: 1},
			},
			ByExpectedMeter: map[string]MeterStats{
				"broken_links": {Total: 1, Hits: 1, HitRate: 1},
			},
			ByMutation: map[string]MeterStats{
				"mut": {Total: 1, Hits: 1, HitRate: 1},
			},
		},
		Results: []Result{{
			RunID:          "adv-test",
			RepoID:         "repo",
			Iteration:      1,
			MutationID:     "mut",
			ExpectedMeters: []string{"broken_links"},
			ActualMeters:   []string{"broken_links"},
			Classification: ClassificationHit,
		}},
	}
	dir, err := WriteReport(root, report)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"iterations.jsonl",
		"summary.json",
		"clusters.md",
		"refinements.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".coherence", "adversarial", "leaderboard.json")); err != nil {
		t.Fatalf("missing leaderboard: %v", err)
	}
	line, err := os.ReadFile(filepath.Join(dir, "iterations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var iter map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(line))), &iter); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"false_negatives", "false_positives", "cluster_key", "duration_ms", "error"} {
		if _, ok := iter[field]; !ok {
			t.Fatalf("iterations.jsonl missing required field %q: %s", field, line)
		}
	}
	for _, field := range []string{"expected_meters", "actual_meters", "false_negatives", "false_positives"} {
		if _, ok := iter[field].([]any); !ok {
			t.Fatalf("iterations.jsonl field %q = %T, want JSON array: %s", field, iter[field], line)
		}
	}
	var lb leaderboard
	data, err := os.ReadFile(filepath.Join(root, ".coherence", "adversarial", "leaderboard.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &lb); err != nil {
		t.Fatal(err)
	}
	if len(lb.Runs) != 1 || lb.Runs[0].RunID != "adv-test" {
		t.Fatalf("leaderboard runs=%+v, want adv-test", lb.Runs)
	}
	if points := lb.ByExpectedMeter["broken_links"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard meter points=%+v, want broken_links hit rate 1", points)
	}
	if points := lb.ByMeter["broken_links"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard by-meter points=%+v, want broken_links hit rate 1", points)
	}
	if points := lb.ByMutation["mut"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard mutation points=%+v, want mut hit rate 1", points)
	}
	md := renderMarkdown(report)
	if !strings.Contains(md, "## Mutation Results") || !strings.Contains(md, "`mut`") {
		t.Fatalf("markdown missing mutation results:\n%s", md)
	}
	loaded, err := LoadReport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RunID != report.RunID {
		t.Fatalf("loaded run=%q, want %q", loaded.RunID, report.RunID)
	}
	if _, err := WriteReport(root, report); err == nil {
		t.Fatal("expected duplicate run report write to fail")
	}
}

func TestWriteReportRejectsUnsafeRunID(t *testing.T) {
	root := t.TempDir()
	_, err := WriteReport(root, Report{RunID: "../outside", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "unsafe run id") {
		t.Fatalf("WriteReport err=%v, want unsafe run id", err)
	}
}

func TestWriteReportRejectsSymlinkedCoherenceDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".coherence")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReport(root, Report{RunID: "adv-safe", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "outside repo root") {
		t.Fatalf("WriteReport err=%v, want symlink escape rejection", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "adversarial")); !os.IsNotExist(statErr) {
		t.Fatalf("outside adversarial dir stat err=%v, want not exists", statErr)
	}
}

func TestWriteReportRejectsSymlinkedLeaderboard(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	base := filepath.Join(root, ".coherence", "adversarial")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "leaderboard.json")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(base, "leaderboard.json")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReport(root, Report{RunID: "adv-leaderboard-safe", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("WriteReport err=%v, want leaderboard symlink rejection", err)
	}
	data, readErr := os.ReadFile(outsideFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "outside\n" {
		t.Fatalf("outside leaderboard target was modified: %q", data)
	}
}

func TestExportMarkdownStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	report := Report{RunID: "adv-export", GeneratedAt: "2026-05-25T00:00:00Z"}
	dst, err := ExportMarkdown(root, "docs/adversarial.md", report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dst, root+string(filepath.Separator)) {
		t.Fatalf("export path=%q, want under root %q", dst, root)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "../outside.md", report); err == nil {
		t.Fatal("expected escaping relative export path to fail")
	}
	if _, err := ExportMarkdown(root, filepath.Join(filepath.Dir(root), "outside.md"), report); err == nil {
		t.Fatal("expected escaping absolute export path to fail")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked-docs")); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "linked-docs/adversarial.md", report); err == nil {
		t.Fatal("expected symlinked export directory to fail")
	}
	if err := os.MkdirAll(filepath.Join(root, "safe-docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "safe-docs", "target.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "safe-docs/target.md", report); err == nil {
		t.Fatal("expected symlinked export file to fail")
	}
}

type fakeHTTPClient struct {
	body string
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

func randForTest() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

func firstLLMSpec() (Spec, bool) {
	for _, spec := range BuiltinSpecs() {
		if spec.RequiresLLM {
			return spec, true
		}
	}
	return Spec{}, false
}

func findResult(results []Result, mutationID string) *Result {
	for i := range results {
		if results[i].MutationID == mutationID {
			return &results[i]
		}
	}
	return nil
}

func resultSignatures(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, strings.Join([]string{
			r.RepoID,
			r.MutationID,
			r.TargetNode.ID,
			strings.Join(r.ExpectedMeters, ","),
			strings.Join(r.ActualMeters, ","),
			r.Classification,
			strings.Join(r.FalseNegatives, ","),
			strings.Join(r.FalsePositives, ","),
			r.SkipReason,
			r.Error,
		}, "|"))
	}
	return out
}
