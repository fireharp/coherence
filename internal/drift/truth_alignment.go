package drift

import (
	"sort"
	"strings"

	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/snapshot"
)

// TruthConflict is one doc-vs-artifact drift pair that needs human
// arbitration. The meter reports structure only; it does not decide which
// side is correct.
type TruthConflict struct {
	Direction          string `json:"direction"`
	AuthorityDoc       string `json:"authority_doc"`
	AuthorityID        string `json:"authority_id"`
	Artifact           string `json:"artifact"`
	ArtifactKind       string `json:"artifact_kind"`
	Relation           string `json:"relation"`
	Question           string `json:"question"`
	IfArtifactIsTruth  string `json:"if_artifact_is_truth"`
	IfAuthorityIsTruth string `json:"if_authority_is_truth"`
}

// TruthAlignment flags linked authority docs and implementation artifacts that
// moved on opposite sides of the coherence baseline.
type TruthAlignment struct {
	Score                 int             `json:"score"`
	RequiresClarification bool            `json:"requires_clarification"`
	Conflicts             []TruthConflict `json:"conflicts"`
}

type truthRelation struct {
	authorityDoc string
	authorityID  string
	artifact     string
	artifactKind string
	relation     string
}

func computeTruthAlignment(baseGraph *graph.Graph, current graph.Graph, baseSnap *snapshot.Snapshot, currentSnap snapshot.Snapshot) TruthAlignment {
	if baseGraph == nil || baseSnap == nil {
		return TruthAlignment{Conflicts: []TruthConflict{}}
	}

	currentFiles := snapshotByPath(currentSnap)
	baseFiles := snapshotByPath(*baseSnap)
	docIDs, docAuthority := truthAuthorityDocs(current)
	symbolPaths := truthCodeSymbolPaths(current)

	relations := []truthRelation{}
	sourceAuthorities := map[string][]truthRelation{}
	add := func(rel truthRelation) {
		if rel.authorityDoc == "" || rel.authorityID == "" || rel.artifact == "" || rel.artifactKind == "" {
			return
		}
		relations = append(relations, rel)
		if rel.artifactKind == "code" {
			sourceAuthorities[rel.artifact] = append(sourceAuthorities[rel.artifact], rel)
		}
	}

	for _, e := range current.Edges {
		switch {
		case e.Kind == graph.EdgeImplements && isTruthTypedID(e.To):
			for _, doc := range docIDs[e.To] {
				artifact := symbolPaths[e.From]
				kind, ok := truthArtifactKind(artifact, currentFiles)
				if ok {
					add(truthRelation{authorityDoc: doc, authorityID: e.To, artifact: artifact, artifactKind: kind, relation: "implements"})
				}
			}
		case e.Kind == graph.EdgeMentions && strings.HasPrefix(e.From, "file:") && isTruthTypedID(e.To):
			artifact := strings.TrimPrefix(e.From, "file:")
			kind, ok := truthArtifactKind(artifact, currentFiles)
			if !ok {
				continue
			}
			for _, doc := range docIDs[e.To] {
				add(truthRelation{authorityDoc: doc, authorityID: e.To, artifact: artifact, artifactKind: kind, relation: "mentions"})
			}
		case e.Kind == graph.EdgeMentions && strings.HasPrefix(e.From, "doc:") && strings.HasPrefix(e.To, "file:"):
			doc := strings.TrimPrefix(e.From, "doc:")
			artifact := strings.TrimPrefix(e.To, "file:")
			kind, ok := truthArtifactKind(artifact, currentFiles)
			if !ok {
				continue
			}
			id := docAuthority[doc]
			if id == "" {
				id = graph.DocNodeID(doc)
			}
			add(truthRelation{authorityDoc: doc, authorityID: id, artifact: artifact, artifactKind: kind, relation: "doc_link"})
		}
	}

	for _, e := range current.Edges {
		if e.Kind != graph.EdgeVerifies || !strings.HasPrefix(e.From, "test:") || !strings.HasPrefix(e.To, "file:") {
			continue
		}
		testPath := strings.TrimPrefix(e.From, "test:")
		sourcePath := strings.TrimPrefix(e.To, "file:")
		for _, rel := range sourceAuthorities[sourcePath] {
			add(truthRelation{
				authorityDoc: rel.authorityDoc,
				authorityID:  rel.authorityID,
				artifact:     testPath,
				artifactKind: "test",
				relation:     "verifies",
			})
		}
	}

	conflicts := []TruthConflict{}
	seen := map[string]bool{}
	for _, rel := range relations {
		authChanged, authOK := truthSemanticChanged(rel.authorityDoc, baseFiles, currentFiles)
		artifactChanged, artifactOK := truthSemanticChanged(rel.artifact, baseFiles, currentFiles)
		if !authOK || !artifactOK || authChanged == artifactChanged {
			continue
		}
		conflict := truthConflict(rel, authChanged, artifactChanged)
		key := conflict.Direction + "|" + conflict.AuthorityDoc + "|" + conflict.AuthorityID + "|" + conflict.Artifact + "|" + conflict.Relation
		if seen[key] {
			continue
		}
		seen[key] = true
		conflicts = append(conflicts, conflict)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Direction != conflicts[j].Direction {
			return conflicts[i].Direction < conflicts[j].Direction
		}
		if conflicts[i].AuthorityDoc != conflicts[j].AuthorityDoc {
			return conflicts[i].AuthorityDoc < conflicts[j].AuthorityDoc
		}
		if conflicts[i].Artifact != conflicts[j].Artifact {
			return conflicts[i].Artifact < conflicts[j].Artifact
		}
		return conflicts[i].Relation < conflicts[j].Relation
	})

	return TruthAlignment{
		Score:                 len(conflicts),
		RequiresClarification: len(conflicts) > 0,
		Conflicts:             conflicts,
	}
}

func truthAuthorityDocs(g graph.Graph) (map[string][]string, map[string]string) {
	byID := map[string][]string{}
	byDoc := map[string]string{}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeDefines || !strings.HasPrefix(e.From, "doc:") || !isTruthTypedID(e.To) {
			continue
		}
		doc := strings.TrimPrefix(e.From, "doc:")
		byID[e.To] = append(byID[e.To], doc)
		if byDoc[doc] == "" {
			byDoc[doc] = e.To
		}
	}
	for id := range byID {
		sort.Strings(byID[id])
	}
	return byID, byDoc
}

func truthCodeSymbolPaths(g graph.Graph) map[string]string {
	paths := map[string]string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NodeCodeSymbol && n.Path != "" {
			paths[n.ID] = n.Path
		}
	}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeDefines && strings.HasPrefix(e.From, "file:") && strings.HasPrefix(e.To, "code_symbol:") {
			if paths[e.To] == "" {
				paths[e.To] = strings.TrimPrefix(e.From, "file:")
			}
		}
	}
	return paths
}

func truthArtifactKind(path string, current map[string]snapshot.FileEntry) (string, bool) {
	if path == "" {
		return "", false
	}
	if graph.IsTestFile(path) {
		return "test", true
	}
	f, ok := current[path]
	if !ok || f.Kind != snapshot.KindCode {
		return "", false
	}
	return "code", true
}

func truthSemanticChanged(path string, base, current map[string]snapshot.FileEntry) (bool, bool) {
	c, ok := current[path]
	if !ok {
		return false, false
	}
	b, ok := base[path]
	if !ok {
		return true, true
	}
	return b.SemanticHash != c.SemanticHash, true
}

func snapshotByPath(s snapshot.Snapshot) map[string]snapshot.FileEntry {
	out := map[string]snapshot.FileEntry{}
	for _, f := range s.Files {
		out[f.Path] = f
	}
	return out
}

func isTruthTypedID(id string) bool {
	return strings.HasPrefix(id, "us:") || strings.HasPrefix(id, "adr:") || strings.HasPrefix(id, "idr:")
}

func truthConflict(rel truthRelation, authChanged, artifactChanged bool) TruthConflict {
	direction := "truth_ahead"
	if artifactChanged && !authChanged {
		direction = "implementation_ahead"
	}
	question := "Should " + rel.artifactKind + " " + rel.artifact + " follow " + rel.authorityID + " in " + rel.authorityDoc + ", or did the artifact intentionally supersede the authority doc?"
	if direction == "implementation_ahead" {
		question = "Did " + rel.artifactKind + " " + rel.artifact + " intentionally supersede " + rel.authorityID + " in " + rel.authorityDoc + ", or should the authority doc remain the source of truth?"
	}
	return TruthConflict{
		Direction:          direction,
		AuthorityDoc:       rel.authorityDoc,
		AuthorityID:        rel.authorityID,
		Artifact:           rel.artifact,
		ArtifactKind:       rel.artifactKind,
		Relation:           rel.relation,
		Question:           question,
		IfArtifactIsTruth:  truthArtifactAction(rel),
		IfAuthorityIsTruth: truthAuthorityAction(rel),
	}
}

func truthArtifactAction(rel truthRelation) string {
	if rel.artifactKind == "test" {
		return "confirm the test captures intended behavior, then update " + rel.authorityDoc + " and any linked code to match"
	}
	return "update " + rel.authorityDoc + " so " + rel.authorityID + " describes the behavior in " + rel.artifact
}

func truthAuthorityAction(rel truthRelation) string {
	if rel.artifactKind == "test" {
		return "fix " + rel.artifact + " so its assertions match " + rel.authorityDoc
	}
	return "fix " + rel.artifact + " so it matches " + rel.authorityDoc
}
