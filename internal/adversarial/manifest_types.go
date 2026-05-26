package adversarial

// Manifest is the corpus manifest accepted by --corpus-manifest.
type Manifest struct {
	Version int         `yaml:"version" json:"version"`
	Repos   []RepoEntry `yaml:"repos" json:"repos"`
}

// RepoEntry describes one local repo in the adversarial corpus.
type RepoEntry struct {
	ID      string   `yaml:"id" json:"id"`
	Path    string   `yaml:"path" json:"path"`
	Tags    []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Weight  int      `yaml:"weight,omitempty" json:"weight,omitempty"`
	Include []string `yaml:"include,omitempty" json:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

type corpusRepo struct {
	RepoEntry
	Files map[string]string
}

// TaxonomyFile is the optional external mutation catalog.
type TaxonomyFile struct {
	Version  int    `yaml:"version" json:"version"`
	Mutation []Spec `yaml:"mutations" json:"mutations"`
}
