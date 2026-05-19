Yes — this is already an active devtools category. What you want is not “new from nothing”; it is more like combining **code intelligence + repo graph + GraphRAG + docs/requirements traceability + incremental indexing**.

For ZenDash specifically, the repo already has the right anchor: docs as the operating spine, role guides, artifact packages, `tools/meta-stats`, generated dashboard fixtures, and `make data-metrics-check` for metric coherence.  The missing layer is a small **repo knowledge base / graph overlay**.

## Closest existing projects

| Project / family                         | What it does                                                                                                                                                            | Fit for your idea                                                                                                                              |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| **Sourcebot**                            | Self-hosted code understanding platform. Has natural-language “Ask Sourcebot” over a codebase, using code search/navigation with inline citations.                      | Very close for “ask my repo questions.” Could be used immediately as the repo Q&A/search layer. ([sourcebot.dev][1])                           |
| **Potpie**                               | Parses a repository into a Neo4j knowledge graph with files, functions, classes, and relationships; agents query that graph for Q&A/debugging/tasks.                    | Very close to “agents living on top of repo graph.” Heavier than your minimal Merkle/pre-commit idea, but directionally aligned. ([GitHub][2]) |
| **Codebase-Memory MCP**                  | Persistent Tree-sitter-based knowledge graph exposed via MCP; stores repo structure in SQLite; paper describes incremental re-indexing via watcher.                     | Probably the closest to your local-first “repo memory for agents” concept. Worth testing. ([GitHub][3])                                        |
| **CocoIndex Code**                       | Open-source semantic code search for agents; Tree-sitter AST-aware chunks, incremental embeddings, CLI/MCP server.                                                      | Good candidate for “fast local code index” instead of building embeddings/chunking yourself. ([CocoIndex][4])                                  |
| **zilliztech/claude-context**            | Code search MCP for Claude Code; claims hybrid search, AST-aware chunking, and incremental indexing with Merkle trees.                                                  | Very relevant to your Merkle-tree/change-detection direction. ([GitHub][5])                                                                    |
| **Aider repo map**                       | Builds a repo map with files and key symbols using Tree-sitter; sends compact structure to the LLM.                                                                     | Not a full KB, but the repo-map technique is exactly the lightweight primitive to steal. ([Aider][6])                                          |
| **SCIP / Sourcegraph code intelligence** | Language-agnostic protocol for code indexing, go-to-definition, find references.                                                                                        | More mature semantic-code-indexing substrate. Good schema inspiration; possibly too heavy for your first version. ([scip-code.org][7])         |
| **Kythe / Meta Glean**                   | Large-scale code facts / semantic graph systems. Glean stores facts about symbols, calls, hierarchies, etc.; Kythe is a graph schema for semantic cross-reference data. | Very strong reference architecture. Too heavy to clone, but useful for ontology ideas. ([GitHub][8])                                           |
| **Zoekt / OpenGrok**                     | Fast code search and cross-reference engines. Sourcebot uses Zoekt-like foundations; OpenGrok is a long-running open-source source search/cross-ref tool.               | Useful as exact-search fallback. Not enough alone for semantic coherence. ([GitHub][9])                                                        |
| **Microsoft GraphRAG / LightRAG**        | Build knowledge graphs from unstructured documents for better retrieval/reasoning.                                                                                      | Useful for docs, ADRs, stories, evidence packets — less ideal for code structure unless combined with code parsers. ([GitHub][10])             |

## The blunt read

There are many projects for:

```txt
codebase Q&A
code search
Tree-sitter indexing
MCP context servers
repo maps
knowledge graph RAG
```

But there are fewer that do your exact target:

```txt
docs ↔ code ↔ user stories ↔ metrics ↔ evidence ↔ generated artifacts
with Merkle-style incremental drift detection
and repo-housekeeping automation
```

So I would **not** build the whole search/indexing layer from scratch. I would build the ZenDash-specific layer on top.

## Best practical stack for your repo

I’d evaluate this combination:

```txt
Sourcebot
  for repo-wide Q&A / code search UI

Codebase-Memory MCP or CocoIndex Code
  for local agent structural context

Your own tiny .repo-kb layer
  for ZenDash-specific ontology:
  docs, ADRs, user stories, metrics, evidence, generated fixtures, commands

Merkle/content hashing
  for cheap change detection and pre-commit/watch mode
```

So the custom thing is small:

```txt
.repo-kb/
  ontology.yml
  index.lock.json
  graph.summary.json
  report.md
```

And most hard work is delegated to existing tools.

## What to borrow directly

From **Aider**:

```txt
repo map = compact list of files + symbols + definitions
```

From **Codebase-Memory / CocoIndex / Claude Context**:

```txt
Tree-sitter parsing
MCP server
incremental index
SQLite/local-first storage
function/class/import/call graph
```

From **Sourcebot / Zoekt**:

```txt
fast exact search
cross-repo search
citations back to source files
```

From **GraphRAG / LightRAG**:

```txt
docs/ADR/story/evidence entity graph
community/topic summaries
higher-level concept navigation
```

From **SCIP/Kythe/Glean**:

```txt
symbol/reference/call hierarchy ontology
facts-about-code model
language-agnostic indexing ideas
```

## What I’d build first

Not a full GraphRAG. Not a Neo4j monster. Start with a small wrapper:

```txt
tools/repo-kb scan
tools/repo-kb diff
tools/repo-kb check
tools/repo-kb watch
```

MVP output:

```json
{
  "root_hash": "sha256:...",
  "changed": [
    "tinybird/endpoints/org_summary.pipe"
  ],
  "affected_entities": [
    "metric:success_rate",
    "metric:cost_per_successful_outcome"
  ],
  "affected_docs": [
    "docs/roles/data-analyst.md",
    "docs/specs/metrics.md"
  ],
  "suggested_checks": [
    "make data-metrics-check",
    "pnpm --dir docs check"
  ]
}
```

Then pre-commit can do:

```bash
pnpm repo-kb scan --staged
pnpm repo-kb diff
pnpm repo-kb check --fast
```

And watch mode can do:

```bash
pnpm repo-kb watch
```

## Useful research/resource direction

For papers/ideas, look at **RepoGraph**, **LocAgent**, and **GraphCodeAgent/CodeRAG**. These are explicitly about repository-level graphs and graph-guided agents rather than plain vector search. RepoGraph presents a repository-level code graph for AI software engineering, LocAgent parses codebases into directed heterogeneous graphs for code localization, and CodeRAG/GraphCodeAgent uses requirement/code graphs for repo-level code generation. ([arXiv][11])

For catalog browsing, the **Code Search Guide** is a useful map of standalone code search tools, and Sourcegraph’s `awesome-code-ai` is a broader list of AI coding/search/MCP tools. ([Code Search Guide][12])

## My recommendation

For now, treat your system as:

```txt
ZenDash Repo KB = thin opinionated coherence layer
not a replacement for Sourcebot / Codebase-Memory / CocoIndex
```

The custom value is not “we can parse TypeScript.” Others do that.

The custom value is:

```txt
we know that this metric, this story, this Rill model,
this Tinybird endpoint, this dashboard chart,
this evidence screenshot, and this role-guide paragraph
are supposed to stay coherent together.
```

That part is specific to your self-driving repository concept, and it is worth building.

[1]: https://www.sourcebot.dev/?utm_source=chatgpt.com "Sourcebot | The Code Understanding Tool"
[2]: https://github.com/potpie-ai/potpie?utm_source=chatgpt.com "potpie-ai/potpie: Spec-driven development for ..."
[3]: https://github.com/DeusData/codebase-memory-mcp?utm_source=chatgpt.com "DeusData/codebase-memory-mcp ..."
[4]: https://cocoindex.io/cocoindex-code?utm_source=chatgpt.com "CocoIndex Code — instantly speed up your coding agent"
[5]: https://github.com/zilliztech/claude-context?utm_source=chatgpt.com "zilliztech/claude-context: Code search MCP for ..."
[6]: https://aider.chat/docs/repomap.html?utm_source=chatgpt.com "Repository map"
[7]: https://scip-code.org/?utm_source=chatgpt.com "SCIP Code Intelligence Protocol"
[8]: https://github.com/facebookincubator/glean?utm_source=chatgpt.com "facebookincubator/Glean: System for collecting, deriving ..."
[9]: https://github.com/sourcegraph/zoekt?utm_source=chatgpt.com "sourcegraph/zoekt: Fast trigram based code search"
[10]: https://github.com/microsoft/graphrag?utm_source=chatgpt.com "microsoft/graphrag: A modular graph-based Retrieval- ..."
[11]: https://arxiv.org/abs/2410.14684?utm_source=chatgpt.com "RepoGraph: Enhancing AI Software Engineering with Repository-level Code Graph"
[12]: https://codesearchguide.org/tools?utm_source=chatgpt.com "Code search tools - Code search guide"

