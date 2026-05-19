package graph

import "testing"

func TestExtractSchemaSQLCreateTable(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"schema/users.sql": `CREATE TABLE users (id INT);`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, DataModelNodeID("users")); !ok {
		t.Fatal("expected data_model:users from SQL CREATE TABLE")
	}
	if !hasEdge(g, FileNodeID("schema/users.sql"),
		DataModelNodeID("users"), EdgeDefines) {
		t.Error("missing defines edge file → data_model")
	}
}

func TestExtractSchemaSQLAllVariants(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"schema/all.sql": `
CREATE TABLE IF NOT EXISTS t1 (id INT);
CREATE OR REPLACE VIEW v1 AS SELECT 1;
CREATE TYPE my_enum AS ENUM ('a');
CREATE MATERIALIZED VIEW mv1 AS SELECT 1;
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"t1", "v1", "my-enum", "mv1"} {
		if _, ok := findNode(g, DataModelNodeID(want)); !ok {
			t.Errorf("missing data_model:%s", want)
		}
	}
}

func TestExtractSchemaSQLQualifiedAndQuotedNames(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"schema/q.sql": "CREATE TABLE public.\"Users\" (id INT);\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, DataModelNodeID("users")); !ok {
		t.Error("expected qualified+quoted name to extract as data_model:users")
	}
}

func TestExtractSchemaProtoMessageAndEnum(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"api/profile.proto": `
syntax = "proto3";
message Profile { string id = 1; }
enum Status { UNKNOWN = 0; }
service Greeter { }
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"profile", "status", "greeter"} {
		if _, ok := findNode(g, DataModelNodeID(want)); !ok {
			t.Errorf("missing data_model:%s", want)
		}
	}
}

func TestExtractSchemaGraphQLTypes(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"api/schema.graphql": `
type Profile { userId: ID! }
input ProfileInput { userId: ID! }
interface Node { id: ID! }
enum Status { UNKNOWN }
union Pet = Dog | Cat
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"profile", "profileinput", "node", "status", "pet"} {
		if _, ok := findNode(g, DataModelNodeID(want)); !ok {
			t.Errorf("missing data_model:%s", want)
		}
	}
}

func TestExtractSchemaDedupsAcrossSources(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"api/profile.proto":   "message Profile {}\n",
		"api/schema.graphql":  "type Profile { id: ID! }\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeDataModel && n.ID == DataModelNodeID("profile") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected single deduped data_model, got %d", count)
	}
	edges := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeDefines && e.To == DataModelNodeID("profile") {
			edges++
		}
	}
	if edges != 2 {
		t.Errorf("expected 2 defines edges (proto + graphql), got %d", edges)
	}
}

func TestExtractSchemaSkipsNonSchemaFiles(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"README.md":         "# Look type Foo here\n",
		"config/db.yaml":    "host: localhost\n",
		"src/auth.go":       "package src\ntype Foo struct{}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeDataModel {
			t.Errorf("unexpected data_model from non-schema file: %+v", n)
		}
	}
}
