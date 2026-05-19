package graph

import "testing"

func TestTSEndpointExpressRoutes(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/api/server.ts": `
const app = express();
app.get("/health", (req, res) => res.send("ok"));
app.post("/items", createItem);
app.put("/items/:id", updateItem);
app.delete("/items/:id", deleteItem);
app.patch("/items/:id", patchItem);
app.head("/probes", probe);
app.options("/cors", cors);
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, path string
	}{
		{"GET", "/health"},
		{"POST", "/items"},
		{"PUT", "/items/:id"},
		{"DELETE", "/items/:id"},
		{"PATCH", "/items/:id"},
		{"HEAD", "/probes"},
		{"OPTIONS", "/cors"},
	}
	for _, c := range cases {
		id := EndpointNodeID(c.method, c.path)
		if _, ok := findNode(g, id); !ok {
			t.Errorf("missing TS endpoint %s %s", c.method, c.path)
		}
		if !hasEdge(g, FileNodeID("src/api/server.ts"), id, EdgeDefines) {
			t.Errorf("missing defines edge for %s %s", c.method, c.path)
		}
	}
}

func TestTSEndpointTemplateLiteralPath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/api.ts": "const app = fastify();\napp.get(`/users`, handler);\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EndpointNodeID("GET", "/users")); !ok {
		t.Error("template-literal path should be captured")
	}
}

func TestTSEndpointDynamicPathSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/api.ts": `
const PREFIX = "/api";
app.get(PREFIX + "/items", handler);
app.post(getPath(), handler);
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint && n.Path == "src/api.ts" {
			t.Errorf("dynamic path should not emit endpoint: %+v", n)
		}
	}
}

func TestTSEndpointSkipsURLSearchParamsAndMapGet(t *testing.T) {
	// Real-world false positive: URLSearchParams.get("foo") and
	// headers.get("Content-Type") were being captured as endpoints
	// because the regex didn't require a leading slash on the path or
	// a second argument (the handler). Tighten so neither matches.
	dir := gitInit(t, map[string]string{
		"src/util.ts": `
const params = new URLSearchParams(location.search);
const debug = params.get("debugMic") === "raw";
const ct = headers.get("Content-Type");
const stored = cache.get("user:42");
const real = app.get("/real", handler);
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint {
			if n.Label != "GET /real" {
				t.Errorf("unexpected endpoint emitted: %+v", n)
			}
		}
	}
}

func TestTSEndpointUseAndAllSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/middleware.ts": `
app.use("/api", middleware);
app.all("/wildcard", handler);
app.any("/", handler);
app.get("/real", handler);
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EndpointNodeID("GET", "/real")); !ok {
		t.Error("real GET should be captured")
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint && n.Label != "GET /real" {
			t.Errorf("non-method-verb call should not emit endpoint: %+v", n)
		}
	}
}

func TestPyEndpointFastAPIDecorators(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/api.py": `from fastapi import FastAPI
app = FastAPI()

@app.get("/health")
def health():
    return {"ok": True}

@app.post("/items")
def create_item():
    pass

@app.put("/items/{id}")
def update(id):
    pass

@app.delete("/items/{id}")
def delete(id):
    pass

@app.patch("/items/{id}")
def patch(id):
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, path string
	}{
		{"GET", "/health"},
		{"POST", "/items"},
		{"PUT", "/items/{id}"},
		{"DELETE", "/items/{id}"},
		{"PATCH", "/items/{id}"},
	}
	for _, c := range cases {
		if _, ok := findNode(g, EndpointNodeID(c.method, c.path)); !ok {
			t.Errorf("missing py endpoint %s %s", c.method, c.path)
		}
	}
}

func TestPyEndpointFlaskRouteDefaultsToWildcard(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/api.py": `from flask import Flask
app = Flask(__name__)

@app.route("/health")
def health():
    return "ok"
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EndpointNodeID("*", "/health")); !ok {
		t.Error("flask @app.route without methods= should emit catch-all *")
	}
}

func TestPyEndpointFlaskRouteExplicitMethods(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/api.py": `
@app.route("/items", methods=["GET", "POST"])
def items():
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		if _, ok := findNode(g, EndpointNodeID(method, "/items")); !ok {
			t.Errorf("missing endpoint %s /items from methods=[]", method)
		}
	}
	if _, ok := findNode(g, EndpointNodeID("*", "/items")); ok {
		t.Error("explicit methods= should not also emit catch-all *")
	}
}

func TestPyEndpointDynamicPathSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/api.py": `PREFIX = "/api"

@app.get(PREFIX + "/items")
def items():
    pass

@app.post(get_path())
def post_items():
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint && n.Path == "app/api.py" {
			t.Errorf("dynamic path should not emit endpoint: %+v", n)
		}
	}
}

func TestPyEndpointInsideCommentsAndDocstringsIgnored(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/api.py": `"""
@app.get("/fake-from-docstring")
def fake():
    pass
"""

# @app.get("/fake-from-comment")
# def fake_too():
#     pass

@app.get("/real")
def real():
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EndpointNodeID("GET", "/real")); !ok {
		t.Error("missing /real endpoint")
	}
	for _, fake := range []string{"/fake-from-docstring", "/fake-from-comment"} {
		if _, ok := findNode(g, EndpointNodeID("GET", fake)); ok {
			t.Errorf("comment/docstring leaked endpoint %s", fake)
		}
	}
}
