package adversarial

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

func clusterResults(results []Result) []Cluster {
	byKey := map[string]*Cluster{}
	for _, r := range results {
		if r.ClusterKey == "" || r.Classification == ClassificationHit || r.Classification == ClassificationSkipped {
			continue
		}
		c := byKey[r.ClusterKey]
		if c == nil {
			c = &Cluster{Key: r.ClusterKey}
			byKey[r.ClusterKey] = c
		}
		c.Count++
		c.MutationIDs = appendUnique(c.MutationIDs, r.MutationID)
		c.ExpectedMeters = appendUnique(c.ExpectedMeters, r.ExpectedMeters...)
		c.ActualMeters = appendUnique(c.ActualMeters, r.ActualMeters...)
		if r.TargetNode.Kind != "" {
			c.TargetKinds = appendUnique(c.TargetKinds, string(r.TargetNode.Kind))
		}
		if r.Error != "" {
			c.ErrorClasses = appendUnique(c.ErrorClasses, errorClass(r.Error))
		}
	}
	out := make([]Cluster, 0, len(byKey))
	for _, c := range byKey {
		sort.Strings(c.MutationIDs)
		sort.Strings(c.ExpectedMeters)
		sort.Strings(c.ActualMeters)
		sort.Strings(c.TargetKinds)
		sort.Strings(c.ErrorClasses)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func clusterKey(r Result, spec Spec) string {
	ext := strings.ToLower(filepath.Ext(r.TargetNode.Path))
	parts := []string{
		spec.Operation,
		string(r.TargetNode.Kind),
		strings.Join(sortedCopy(r.ExpectedMeters), ","),
		strings.Join(sortedCopy(r.ActualMeters), ","),
		ext,
		extractorFamily(r.TargetNode.Path),
		errorClass(r.Error),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

func extractorFamily(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mts", ".cts":
		return "typescript"
	case ".py":
		return "python"
	case ".md", ".markdown":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql", ".proto", ".graphql":
		return "schema"
	default:
		return "generic"
	}
}

func errorClass(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, ':'); i > 0 {
		return s[:i]
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "error"
	}
	return fields[0]
}
