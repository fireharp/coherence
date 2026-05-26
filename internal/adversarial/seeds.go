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
	return mergeEmbeddedFiles(
		embeddedBaseFiles(),
		embeddedGoFiles(),
		embeddedPolyglotRiskFiles(),
		embeddedMetricFrontendFiles(),
		embeddedTSJSFiles(),
		embeddedPythonFiles(),
		embeddedDocFiles(),
	)
}

func mergeEmbeddedFiles(groups ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, group := range groups {
		for path, content := range group {
			out[path] = content
		}
	}
	return out
}
