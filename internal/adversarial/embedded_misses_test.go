package adversarial

import "testing"

func assertEmbeddedMisses(t *testing.T, results []Result) {
	t.Helper()
	for _, tc := range []struct {
		mutationID string
		meter      string
	}{
		{mutationID: "ADV-022-agent-skill-unknown-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-023-split-string-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-024-dynamic-ts-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-025-python-dynamic-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-026-raw-adr-citation-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-027-reference-style-link-demo", meter: "broken_links"},
		{mutationID: "ADV-028-ts-reexport-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-029-html-markdown-link-demo", meter: "broken_links"},
		{mutationID: "ADV-030-python-dynamic-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-031-ts-dynamic-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-032-go-dynamic-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-033-python-absolute-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-034-ts-path-alias-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-035-python-import-statement-demo", meter: "dangling_imports"},
		{mutationID: "ADV-036-reference-style-adr-citation-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-037-mdx-user-story-demo", meter: "unimplemented_stories"},
		{mutationID: "ADV-038-metric-measure-name-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-039-ts-tests-dir-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-040-mdx-broken-link-demo", meter: "broken_links"},
		{mutationID: "ADV-041-ts-require-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-042-ts-triple-slash-reference-demo", meter: "dangling_imports"},
		{mutationID: "ADV-043-python-tests-dir-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-044-python-dotted-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-045-markdown-wiki-link-demo", meter: "broken_links"},
		{mutationID: "ADV-046-ts-import-equals-require-demo", meter: "dangling_imports"},
		{mutationID: "ADV-047-ts-multiline-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-048-agent-doc-unknown-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-049-markdown-extension-link-demo", meter: "broken_links"},
		{mutationID: "ADV-050-vue-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-051-css-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-052-fastapi-add-api-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-053-quoted-code-typed-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-054-mdx-metric-prop-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-055-go-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-056-markdown-angle-autolink-demo", meter: "broken_links"},
		{mutationID: "ADV-057-go-integration-test-stale-demo", meter: "stale_tests"},
		{mutationID: "ADV-058-adr-capitalized-supersedes-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-059-ts-route-chain-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-060-svelte-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-061-next-route-handler-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-062-ts-dependency-cycle-demo", meter: "dependency_cycles"},
		{mutationID: "ADV-063-quoted-user-story-id-demo", meter: "unimplemented_stories"},
		{mutationID: "ADV-064-rust-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-065-json-typed-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-066-yaml-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-067-markdown-title-link-demo", meter: "broken_links"},
		{mutationID: "ADV-068-go-gin-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-069-adr-quoted-supersedes-key-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-070-ts-test-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-071-ruby-rails-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-072-toml-typed-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-073-toml-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-074-openapi-path-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-075-java-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-076-yaml-user-story-demo", meter: "unimplemented_stories"},
		{mutationID: "ADV-077-python-file-cycle-demo", meter: "dependency_cycles"},
		{mutationID: "ADV-078-markdown-angle-destination-space-demo", meter: "broken_links"},
		{mutationID: "ADV-079-python-from-dot-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-080-django-urlconf-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-081-go-unused-method-dead-code-demo", meter: "dead_code"},
		{mutationID: "ADV-082-template-literal-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-083-production-scenario-typed-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-084-csharp-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-085-markdown-collapsed-reference-link-demo", meter: "broken_links"},
		{mutationID: "ADV-086-adr-nested-supersedes-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-087-spring-getmapping-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-088-rule-trigger-deletion-demo", meter: "required_edge_breakage"},
		{mutationID: "ADV-089-numbered-claim-support-demo", meter: "claim_support"},
		{mutationID: "ADV-090-makefile-include-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-091-h3-concept-path-loss-demo", meter: "path_loss"},
		{mutationID: "ADV-092-shell-source-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-093-go-mux-handlefunc-methods-endpoint-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-094-mjs-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-095-setext-concept-path-loss-demo", meter: "path_loss"},
		{mutationID: "ADV-096-js-esm-dangling-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-097-ts-optional-chain-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-098-ts-bracket-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-099-graphql-import-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-100-dockerfile-copy-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-101-package-script-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-102-go-embed-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-103-compose-env-file-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-104-bazel-load-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-105-jupyter-import-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-106-github-action-local-uses-demo", meter: "dangling_imports"},
		{mutationID: "ADV-107-terraform-module-source-demo", meter: "dangling_imports"},
		{mutationID: "ADV-108-kustomize-resource-demo", meter: "dangling_imports"},
		{mutationID: "ADV-109-asciidoc-xref-demo", meter: "broken_links"},
		{mutationID: "ADV-110-kotlin-ktor-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-111-commonjs-require-demo", meter: "dangling_imports"},
		{mutationID: "ADV-112-tsconfig-reference-demo", meter: "dangling_imports"},
		{mutationID: "ADV-113-nestjs-decorator-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-114-helm-template-include-demo", meter: "dangling_imports"},
		{mutationID: "ADV-115-rst-local-link-demo", meter: "broken_links"},
		{mutationID: "ADV-116-avro-schema-ref-demo", meter: "dangling_imports"},
		{mutationID: "ADV-117-e2e-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-118-static-html-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-119-mermaid-click-link-demo", meter: "broken_links"},
		{mutationID: "ADV-120-rust-mod-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-121-aspnet-minimal-api-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-122-cargo-workspace-member-demo", meter: "dangling_imports"},
		{mutationID: "ADV-123-markdown-shortcut-reference-demo", meter: "broken_links"},
		{mutationID: "ADV-124-blockquote-claim-support-demo", meter: "claim_support"},
		{mutationID: "ADV-125-table-claim-support-demo", meter: "claim_support"},
		{mutationID: "ADV-126-uppercase-story-frontmatter-demo", meter: "unimplemented_stories"},
		{mutationID: "ADV-127-go-servemux-method-pattern-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-128-protobuf-import-demo", meter: "dangling_imports"},
		{mutationID: "ADV-129-markdown-task-list-claim-demo", meter: "claim_support"},
		{mutationID: "ADV-130-github-reusable-workflow-demo", meter: "dangling_imports"},
		{mutationID: "ADV-131-kotlin-stale-test-demo", meter: "stale_tests"},
		{mutationID: "ADV-132-laravel-route-demo", meter: "orphan_endpoints"},
		{mutationID: "ADV-133-openapi-local-ref-demo", meter: "dangling_imports"},
		{mutationID: "ADV-134-asciidoc-user-story-demo", meter: "unimplemented_stories"},
		{mutationID: "ADV-135-gitlab-ci-include-demo", meter: "dangling_imports"},
		{mutationID: "ADV-136-markdown-table-semantic-demo", meter: "semantic_movement"},
		{mutationID: "ADV-137-csv-metric-alias-demo", meter: "orphaned_metric_aliases"},
		{mutationID: "ADV-138-markdown-footnote-link-demo", meter: "broken_links"},
		{mutationID: "ADV-139-toml-adr-supersedes-demo", meter: "stale_decision_links"},
		{mutationID: "ADV-140-sql-double-quoted-typed-id-demo", meter: "unknown_id_references"},
		{mutationID: "ADV-142-mkdocs-nav-missing-page-demo", meter: "broken_links"},
		{mutationID: "ADV-143-docusaurus-sidebar-missing-doc-demo", meter: "broken_links"},
		{mutationID: "ADV-144-nginx-include-dangling-demo", meter: "dangling_imports"},
		{mutationID: "ADV-145-systemd-environment-file-demo", meter: "dangling_imports"},
		{mutationID: "ADV-147-json-asset-bare-import-demo", meter: "dangling_imports"},
	} {
		t.Run(tc.mutationID, func(t *testing.T) {
			res := findResult(results, tc.mutationID)
			if res == nil {
				t.Fatalf("missing %s exploration demo result", tc.mutationID)
			}
			if res.Classification != ClassificationMiss ||
				len(res.FalseNegatives) != 1 ||
				res.FalseNegatives[0] != tc.meter ||
				len(res.FalsePositives) != 0 {
				t.Fatalf("%s result=%+v, want false negative for %s", tc.mutationID, *res, tc.meter)
			}
		})
	}
}

func assertEmbeddedFalsePositives(t *testing.T, results []Result) {
	t.Helper()
	for _, tc := range []struct {
		mutationID string
		meter      string
	}{
		{mutationID: "ADV-146-html-anchor-support-path-loss-fp-demo", meter: "path_loss"},
	} {
		t.Run(tc.mutationID, func(t *testing.T) {
			res := findResult(results, tc.mutationID)
			if res == nil {
				t.Fatalf("missing %s exploration demo result", tc.mutationID)
			}
			if res.Classification != ClassificationFP ||
				len(res.FalseNegatives) != 0 ||
				len(res.FalsePositives) != 1 ||
				res.FalsePositives[0] != tc.meter {
				t.Fatalf("%s result=%+v, want false positive for %s", tc.mutationID, *res, tc.meter)
			}
		})
	}
}
