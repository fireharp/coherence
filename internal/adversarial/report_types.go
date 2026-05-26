package adversarial

// Report is the JSON output from one adversarial run.
type Report struct {
	RunID       string       `json:"run_id"`
	GeneratedAt string       `json:"generated_at"`
	Seed        int64        `json:"seed"`
	Iterations  int          `json:"iterations"`
	Pass        bool         `json:"pass"`
	Strict      bool         `json:"strict"`
	RefineFrom  string       `json:"refine_from,omitempty"`
	Repos       []string     `json:"repos"`
	Specs       []string     `json:"specs"`
	LLMSpecs    LLMExpansion `json:"llm_specs"`
	Summary     Summary      `json:"summary"`
	Clusters    []Cluster    `json:"clusters"`
	Refinements []Refinement `json:"refinements"`
	Results     []Result     `json:"results"`
	ReportDir   string       `json:"report_dir,omitempty"`
	ExportPath  string       `json:"export_path,omitempty"`
	NextCommand string       `json:"next_command,omitempty"`
}

// LLMExpansion records optional Groq-generated taxonomy expansion status.
type LLMExpansion struct {
	Requested bool   `json:"requested"`
	Enabled   bool   `json:"enabled"`
	Accepted  int    `json:"accepted"`
	Skipped   string `json:"skipped"`
	Error     string `json:"error"`
}

// LoopReport is emitted when the adversarial bench runs multiple refinement
// cycles in one command.
type LoopReport struct {
	GeneratedAt string   `json:"generated_at"`
	Cycles      int      `json:"cycles"`
	Pass        bool     `json:"pass"`
	Strict      bool     `json:"strict"`
	Runs        []Report `json:"runs"`
	Final       Report   `json:"final"`
	NextCommand string   `json:"next_command,omitempty"`
}
