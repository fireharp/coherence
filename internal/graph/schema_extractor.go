package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// extractSchemas scans tracked schema files (SQL DDL, protobuf, GraphQL)
// and emits one data_model node per declared entity. Names dedupe across
// sources via slugified ID (`data_model:<slug>`). `defines` edge from
// each contributing file.
func extractSchemas(b *Builder, rootDir string, tracked []string) {
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		switch ext {
		case ".sql":
			emitSchemaModels(b, rootDir, rel, sqlEntityRe, "sql")
		case ".proto":
			emitSchemaModels(b, rootDir, rel, protoEntityRe, "proto")
		case ".graphql", ".gql":
			emitSchemaModels(b, rootDir, rel, graphqlEntityRe, "graphql")
		}
	}
}

var (
	// SQL: CREATE [OR REPLACE] [TABLE|VIEW|TYPE|MATERIALIZED VIEW]
	// [IF NOT EXISTS] [<schema>.]<name>
	sqlEntityRe = regexp.MustCompile(
		`(?im)^\s*CREATE(?:\s+OR\s+REPLACE)?\s+(?:MATERIALIZED\s+VIEW|TABLE|VIEW|TYPE)\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:[A-Za-z0-9_]+\.)?["` + "`" + `]?([A-Za-z_][A-Za-z0-9_]*)["` + "`" + `]?`)

	// proto: message Foo {  |  enum Foo {  |  service Foo {
	protoEntityRe = regexp.MustCompile(`(?m)^\s*(?:message|enum|service)\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// graphql: type Foo  |  input Foo  |  interface Foo  |  enum Foo  |  union Foo
	graphqlEntityRe = regexp.MustCompile(`(?m)^\s*(?:type|input|interface|enum|union)\s+([A-Za-z_][A-Za-z0-9_]*)`)
)

func emitSchemaModels(b *Builder, rootDir, rel string, re *regexp.Regexp, source string) {
	abs := filepath.Join(rootDir, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, m := range re.FindAllSubmatch(data, -1) {
		name := string(m[1])
		slug := slugify(name)
		if slug == "" {
			continue
		}
		if seen[slug] {
			continue
		}
		seen[slug] = true
		b.AddNode(Node{
			ID:    DataModelNodeID(slug),
			Kind:  NodeDataModel,
			Label: name,
			Meta:  map[string]string{"source_kind": source},
		})
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         DataModelNodeID(slug),
			Kind:       EdgeDefines,
			Provenance: rel + " (" + source + " schema)",
		})
	}
}
