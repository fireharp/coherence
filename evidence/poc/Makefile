.PHONY: all build test clean run-all reset

# All POCs in this directory reproduce a measurement reported in REPORT.md /
# ITERATION-*.md / FINDINGS.md. This Makefile builds them, runs the tests on
# the native go/ast extractor, and re-runs each POC against the indexed
# corpora under ../raw/.

BIN_DIR := $(CURDIR)/bin
SCRATCH := $(CURDIR)/.scratch

NATIVE := $(BIN_DIR)/go_ast_extractor
POC_DEAD := $(BIN_DIR)/cgpoc
POC_DEAD_V2 := $(BIN_DIR)/cgpoc_v2
POC_BLAST := $(BIN_DIR)/cgpoc_blast

# Default corpus pointers — override on the command line:
#   make COHERENCE_DB=/path/to/.codegraph/codegraph.db run-all
COHERENCE_ROOT ?= $(realpath $(CURDIR)/../..)
COHERENCE_DB   ?= $(CURDIR)/../raw/codegraph_coherence_self.db
COPYCAT_DB     ?= $(CURDIR)/../raw/codegraph_copycat.db
TS_DB          ?= $(CURDIR)/../raw/codegraph_ts.db

all: build test ## Build all POCs and run native-extractor tests

# -----------------------------------------------------------------------------
# Build all POC binaries into ./bin/. Each POC is a single Go file that
# expects to live alone in a build directory because they all use `package
# main`. We stage each one in .scratch/<name>/ before building.
# -----------------------------------------------------------------------------

build: $(NATIVE) $(POC_DEAD) $(POC_DEAD_V2) $(POC_BLAST)

$(NATIVE): go_ast_extractor.go go_ast_extractor_test.go | $(BIN_DIR) $(SCRATCH)
	@rm -rf $(SCRATCH)/native && mkdir -p $(SCRATCH)/native
	@cp go_ast_extractor.go go_ast_extractor_test.go $(SCRATCH)/native/
	@cd $(SCRATCH)/native && go mod init native >/dev/null 2>&1 || true
	@cd $(SCRATCH)/native && go build -tags poc -o $@ ./

$(POC_DEAD): cgpoc.go | $(BIN_DIR) $(SCRATCH)
	@$(MAKE) -s _build NAME=cgpoc SRC=cgpoc.go OUT=$@

$(POC_DEAD_V2): cgpoc_v2.go | $(BIN_DIR) $(SCRATCH)
	@$(MAKE) -s _build NAME=cgpoc_v2 SRC=cgpoc_v2.go OUT=$@

$(POC_BLAST): cgpoc_blast.go | $(BIN_DIR) $(SCRATCH)
	@$(MAKE) -s _build NAME=cgpoc_blast SRC=cgpoc_blast.go OUT=$@

# Internal helper: stage one POC in .scratch/<name>/ and build.
_build:
	@rm -rf $(SCRATCH)/$(NAME) && mkdir -p $(SCRATCH)/$(NAME)
	@cp $(SRC) $(SCRATCH)/$(NAME)/main.go
	@cd $(SCRATCH)/$(NAME) && go mod init $(NAME) >/dev/null 2>&1 || true
	@cd $(SCRATCH)/$(NAME) && go get github.com/mattn/go-sqlite3 >/dev/null 2>&1
	@cd $(SCRATCH)/$(NAME) && go build -tags poc -o $(OUT) main.go

$(BIN_DIR) $(SCRATCH):
	@mkdir -p $@

# -----------------------------------------------------------------------------
# Tests (native extractor only — the SQLite-driven POCs don't have unit tests
# because they require an indexed corpus, not synthesizable Go code).
# -----------------------------------------------------------------------------

test: $(NATIVE) ## Run native extractor unit tests
	@cd $(SCRATCH)/native && go test -tags poc ./...

test-smoke: $(NATIVE) ## Run native extractor unit tests including the real-corpus smoke test
	@cd $(SCRATCH)/native && COHERENCE_ROOT=$(COHERENCE_ROOT) go test -tags poc -v ./...

# -----------------------------------------------------------------------------
# Reproduce every JSON output in this directory.
# -----------------------------------------------------------------------------

run-all: build ## Re-run every POC and refresh the canonical JSON outputs
	$(NATIVE) --root=$(COHERENCE_ROOT) > go_ast_extractor_full.json
	$(POC_DEAD)    --db=$(COHERENCE_DB) > dead_code_coherence_self.json
	$(POC_DEAD)    --db=$(COPYCAT_DB)   > dead_code_copycat.json
	$(POC_DEAD)    --db=$(TS_DB)        > dead_code_ts.json
	$(POC_DEAD_V2) --db=$(COHERENCE_DB) > dead_code_v2_coherence_self.json
	$(POC_DEAD_V2) --db=$(COPYCAT_DB)   > dead_code_v2_copycat.json
	$(POC_DEAD_V2) --db=$(TS_DB)        > dead_code_v2_ts.json
	$(POC_BLAST) --db=$(COHERENCE_DB) \
	  --symbols="graph.Build,drift.ComputeWith,drift.computeContradiction,graph.slugify,Build,Compute,Run" \
	  --depth=2 > callsite_blast_radius_coherence_v2.json
	@echo "All POC outputs refreshed."

# -----------------------------------------------------------------------------

clean: ## Remove built binaries and scratch
	rm -rf $(BIN_DIR) $(SCRATCH)

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
