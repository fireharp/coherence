// Package adversarial runs graph-seeded mutation benchmarks against
// coherence's drift meters.
package adversarial

import (
	"net/http"

	"github.com/fireharp/coherence/internal/graph"
)

const (
	ClassificationHit     = "hit"
	ClassificationMiss    = "false_negative"
	ClassificationFP      = "false_positive"
	ClassificationSkipped = "skipped"
	ClassificationErrored = "errored"
)

// Options configures one adversarial benchmark run.
type Options struct {
	RootDir        string
	ManifestPath   string
	TaxonomyPath   string
	RefineFrom     string
	Iterations     int
	Cycles         int
	Seed           int64
	Jobs           int
	LLM            bool
	LLMSpecs       bool
	Strict         bool
	WriteReport    bool
	ExportReport   string
	JSONOut        bool
	GroqEndpoint   string
	GroqHTTPClient HTTPDoer
}

// HTTPDoer is the minimal http.Client surface used by the optional LLM
// mutation-spec generator.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Spec is the mutation DSL. Built-in and LLM-generated specs share this
// exact shape.
type Spec struct {
	ID                         string           `yaml:"id" json:"id"`
	Description                string           `yaml:"description,omitempty" json:"description,omitempty"`
	Operation                  string           `yaml:"operation" json:"operation"`
	TargetKinds                []graph.NodeKind `yaml:"target_kinds,omitempty" json:"target_kinds,omitempty"`
	ExpectedMeters             []string         `yaml:"expected_meters" json:"expected_meters"`
	AllowedSideEffectMeters    []string         `yaml:"allowed_side_effect_meters,omitempty" json:"allowed_side_effect_meters,omitempty"`
	RequiresLLM                bool             `yaml:"requires_llm,omitempty" json:"requires_llm,omitempty"`
	Selector                   Selector         `yaml:"selector,omitempty" json:"selector,omitempty"`
	Edit                       Edit             `yaml:"edit,omitempty" json:"edit,omitempty"`
	SkipConditions             SkipConditions   `yaml:"skip_conditions,omitempty" json:"skip_conditions,omitempty"`
	SkipReasonWhenInapplicable string           `yaml:"skip_reason_when_inapplicable,omitempty" json:"skip_reason_when_inapplicable,omitempty"`
}

// SkipConditions declare explicit preconditions for a mutation. Missing
// preconditions classify the iteration as skipped, not errored.
type SkipConditions struct {
	RequireEnv             []string `yaml:"require_env,omitempty" json:"require_env,omitempty"`
	RequireFiles           []string `yaml:"require_files,omitempty" json:"require_files,omitempty"`
	RequireOptionalEngines []string `yaml:"require_optional_engines,omitempty" json:"require_optional_engines,omitempty"`
}

// Selector narrows graph target candidates for a mutation.
type Selector struct {
	IDPrefix        string   `yaml:"id_prefix,omitempty" json:"id_prefix,omitempty"`
	PathGlob        string   `yaml:"path_glob,omitempty" json:"path_glob,omitempty"`
	PathContains    string   `yaml:"path_contains,omitempty" json:"path_contains,omitempty"`
	PathSuffix      string   `yaml:"path_suffix,omitempty" json:"path_suffix,omitempty"`
	Extensions      []string `yaml:"extensions,omitempty" json:"extensions,omitempty"`
	LabelContains   string   `yaml:"label_contains,omitempty" json:"label_contains,omitempty"`
	HasIncomingEdge string   `yaml:"has_incoming_edge,omitempty" json:"has_incoming_edge,omitempty"`
	HasOutgoingEdge string   `yaml:"has_outgoing_edge,omitempty" json:"has_outgoing_edge,omitempty"`
}

// Edit describes how an operation changes the materialized temp repo.
type Edit struct {
	Path         string `yaml:"path,omitempty" json:"path,omitempty"`
	NewPath      string `yaml:"new_path,omitempty" json:"new_path,omitempty"`
	Old          string `yaml:"old,omitempty" json:"old,omitempty"`
	New          string `yaml:"new,omitempty" json:"new,omitempty"`
	Text         string `yaml:"text,omitempty" json:"text,omitempty"`
	Content      string `yaml:"content,omitempty" json:"content,omitempty"`
	LineContains string `yaml:"line_contains,omitempty" json:"line_contains,omitempty"`
	AgeDays      int    `yaml:"age_days,omitempty" json:"age_days,omitempty"`
}

// Target records the graph node selected for a mutation.
type Target struct {
	ID    string         `json:"id"`
	Kind  graph.NodeKind `json:"kind"`
	Label string         `json:"label,omitempty"`
	Path  string         `json:"path,omitempty"`
}

// Result is one adversarial iteration.
type Result struct {
	RunID          string   `json:"run_id"`
	RepoID         string   `json:"repo_id"`
	Iteration      int      `json:"iteration"`
	MutationID     string   `json:"mutation_id"`
	Hypothesis     string   `json:"hypothesis,omitempty"`
	TargetNode     Target   `json:"target_node"`
	ExpectedMeters []string `json:"expected_meters"`
	ActualMeters   []string `json:"actual_meters"`
	Classification string   `json:"classification"`
	FalseNegatives []string `json:"false_negatives"`
	FalsePositives []string `json:"false_positives"`
	ClusterKey     string   `json:"cluster_key"`
	Refinement     string   `json:"refinement,omitempty"`
	DurationMS     int64    `json:"duration_ms"`
	Error          string   `json:"error"`
	SkipReason     string   `json:"skip_reason,omitempty"`
}

// Cluster groups structurally similar misses and false positives.
type Cluster struct {
	Key            string   `json:"key"`
	Count          int      `json:"count"`
	MutationIDs    []string `json:"mutation_ids"`
	ExpectedMeters []string `json:"expected_meters"`
	ActualMeters   []string `json:"actual_meters"`
	TargetKinds    []string `json:"target_kinds"`
	ErrorClasses   []string `json:"error_classes,omitempty"`
}

// Refinement is the next experiment suggested by observed misses, false
// positives, skips, or errors.
type Refinement struct {
	ClusterKey      string   `json:"cluster_key,omitempty"`
	MutationIDs     []string `json:"mutation_ids,omitempty"`
	Hypothesis      string   `json:"hypothesis"`
	Observation     string   `json:"observation"`
	NextExperiment  string   `json:"next_experiment"`
	SuggestedAction string   `json:"suggested_action"`
	Count           int      `json:"count"`
}

// Summary is the suite-wide roll-up.
type Summary struct {
	Total             int                   `json:"total"`
	Hits              int                   `json:"hits"`
	FalseNegatives    int                   `json:"false_negatives"`
	FalsePositives    int                   `json:"false_positives"`
	Skipped           int                   `json:"skipped"`
	Errored           int                   `json:"errored"`
	HitRate           float64               `json:"hit_rate"`
	FalseNegativeRate float64               `json:"false_negative_rate"`
	FalsePositiveRate float64               `json:"false_positive_rate"`
	ByMeter           map[string]MeterStats `json:"by_meter"`
	ByExpectedMeter   map[string]MeterStats `json:"by_expected_meter"`
	ByMutation        map[string]MeterStats `json:"by_mutation"`
}

// MeterStats records hit/miss counts for one grouping key.
type MeterStats struct {
	Total             int     `json:"total"`
	Hits              int     `json:"hits"`
	FalseNegatives    int     `json:"false_negatives"`
	FalsePositives    int     `json:"false_positives"`
	Skipped           int     `json:"skipped"`
	Errored           int     `json:"errored"`
	HitRate           float64 `json:"hit_rate"`
	FalseNegativeRate float64 `json:"false_negative_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
}
