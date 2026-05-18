// Command repo-kb runs repo-coherence checks against staged or diffed git
// changes. Mirrors the CLI defined in the old bin/repo-kb.mjs.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"repo-kb/internal/git"
	"repo-kb/internal/ids"
	"repo-kb/internal/llm"
	"repo-kb/internal/ontology"
	"repo-kb/internal/report"
	"repo-kb/internal/rules"
	"repo-kb/internal/status"
)

const usage = `repo-kb <subcommand> [flags]
  scan --staged [--llm] [--ontology=path]  evaluate staged files
  check [--ref=HEAD~1] [--ontology=path]   evaluate a diff range
  report                   print the last report stored at .repo-kb/last-report.json
  status [--ontology=path] rewrite .repo-kb/STATUS.md (current state)
env:
  ZEN_REPO_KB_OFF=1        skip all checks, exit 0
  ZEN_REPO_KB_LLM=1        enable LLM semantic pass (requires GROQ_API_KEY)
  ZEN_REPO_KB_GROQ_MODEL   override Groq model id (default: llama-3.3-70b-versatile)
`

type parsedArgs struct {
	flags map[string]any
}

func parseArgs(argv []string) parsedArgs {
	out := parsedArgs{flags: map[string]any{}}
	for _, tok := range argv {
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		body := tok[2:]
		if i := strings.IndexByte(body, '='); i >= 0 {
			out.flags[body[:i]] = body[i+1:]
		} else {
			out.flags[body] = true
		}
	}
	return out
}

func resolveOntologyPath(rootDir string, args parsedArgs) string {
	raw, _ := args.flags["ontology"].(string)
	if raw == "" {
		return filepath.Join(rootDir, "ontology.yml")
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(rootDir, raw)
}

func pickFiles(sub string, args parsedArgs, rootDir string) []string {
	if sub == "scan" {
		if _, ok := args.flags["staged"]; ok {
			return git.StagedFiles(rootDir)
		}
		return []string{}
	}
	if sub == "check" {
		ref, _ := args.flags["ref"].(string)
		if ref == "" {
			ref = "HEAD~1"
		}
		return git.DiffNameOnly(ref, rootDir)
	}
	return []string{}
}

const severityRankError = 2

func severityRank(s string) int {
	switch s {
	case "warn":
		return 1
	case "error":
		return 2
	}
	return 0
}

func runScan(files []string, useLLM bool, rootDir, ontPath string) (int, []rules.Finding, llm.Result, error) {
	ont, err := ontology.Load(ontPath)
	if err != nil {
		return 0, nil, llm.Result{}, err
	}
	ruleFindings := rules.Evaluate(ont, files)

	idIndex := ids.Build(rootDir)
	addedByPath := map[string]string{}
	fileOrder := []string{}
	for _, rel := range files {
		if strings.HasSuffix(rel, ".md") {
			continue
		}
		content := git.StagedAddedContent(rel, rootDir)
		addedByPath[rel] = content
		fileOrder = append(fileOrder, rel)
	}
	idFindings := ids.Scan(addedByPath, fileOrder, idIndex)

	llmResult := llm.Run(files, useLLM, rootDir)

	all := append([]rules.Finding{}, ruleFindings...)
	for _, f := range idFindings {
		all = append(all, rules.Finding{
			Rule: f.Rule, Severity: f.Severity, Message: f.Message,
			TriggeredBy: f.TriggeredBy, ExpectedAnyOf: f.ExpectedAnyOf,
		})
	}
	for _, f := range llmResult.Findings {
		all = append(all, rules.Finding{
			Rule: f.Rule, Severity: f.Severity, Message: f.Message,
			TriggeredBy: f.TriggeredBy, ExpectedAnyOf: f.ExpectedAnyOf,
		})
	}
	return len(ont.Rules), all, llmResult, nil
}

func summarizeFinding(f rules.Finding) string {
	sev := strings.ToUpper(f.Severity)
	head := fmt.Sprintf("[%s] %s: %s", sev, f.Rule, f.Message)
	lines := []string{head}
	if len(f.TriggeredBy) > 0 {
		shown := f.TriggeredBy
		extra := ""
		if len(shown) > 5 {
			extra = fmt.Sprintf(" (+%d more)", len(shown)-5)
			shown = shown[:5]
		}
		lines = append(lines, "  triggered by: "+strings.Join(shown, ", ")+extra)
	}
	if len(f.ExpectedAnyOf) > 0 {
		shown := f.ExpectedAnyOf
		extra := ""
		if len(shown) > 5 {
			extra = fmt.Sprintf(" (+%d more)", len(shown)-5)
			shown = shown[:5]
		}
		lines = append(lines, "  expected any of: "+strings.Join(shown, ", ")+extra)
	}
	return strings.Join(lines, "\n")
}

func printReport(files []string, ruleCount int, findings []rules.Finding, llmRes llm.Result) {
	fmt.Printf("repo-kb: %d file(s), %d rules loaded\n", len(files), ruleCount)
	if llmRes.Skipped != "" {
		fmt.Printf("repo-kb: llm pass skipped (%s)\n", llmRes.Skipped)
	} else {
		fmt.Printf("repo-kb: llm pass via %s, %d call(s)\n", llmRes.Model, llmRes.Calls)
	}
	if len(findings) == 0 {
		fmt.Println("repo-kb: no findings.")
		return
	}
	fmt.Printf("repo-kb: %d finding(s):\n", len(findings))
	for _, f := range findings {
		fmt.Println(summarizeFinding(f))
	}
}

func maxSeverity(findings []rules.Finding) int {
	r := 0
	for _, f := range findings {
		if s := severityRank(f.Severity); s > r {
			r = s
		}
	}
	return r
}

func main() {
	os.Exit(run())
}

func run() int {
	argv := os.Args[1:]
	sub := "scan"
	if len(argv) > 0 {
		sub = argv[0]
		argv = argv[1:]
	}
	args := parseArgs(argv)

	if sub == "help" || sub == "--help" || args.flags["help"] == true {
		fmt.Print(usage)
		return 0
	}

	if os.Getenv("ZEN_REPO_KB_OFF") == "1" {
		fmt.Fprintln(os.Stderr, "repo-kb: ZEN_REPO_KB_OFF=1 set; skipping coherence checks")
		return 0
	}

	rootDir, err := git.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
		return 2
	}
	ontPath := resolveOntologyPath(rootDir, args)

	switch sub {
	case "report":
		p := report.Path(rootDir)
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: no report on disk yet")
			return 0
		}
		defer f.Close()
		if _, err := io.Copy(os.Stdout, f); err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
			return 2
		}
		return 0

	case "status":
		ont, err := ontology.Load(ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
			return 2
		}
		out, err := status.Write(rootDir, ont)
		if err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
			return 2
		}
		rel, err := filepath.Rel(rootDir, out)
		if err != nil {
			rel = out
		}
		fmt.Printf("repo-kb: wrote %s\n", rel)
		return 0

	case "scan", "check":
		files := pickFiles(sub, args, rootDir)
		useLLM, _ := args.flags["llm"].(bool)
		ruleCount, findings, llmRes, err := runScan(files, useLLM, rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
			return 2
		}
		payload := report.Payload{
			Subcommand:  sub,
			Flags:       args.flags,
			Files:       files,
			RuleCount:   ruleCount,
			LLM:         report.FromResult(llmRes),
			Findings:    findings,
			GeneratedAt: report.Now(),
		}
		if err := report.Write(rootDir, payload); err != nil {
			fmt.Fprintln(os.Stderr, "repo-kb: fatal:", err)
			return 2
		}
		printReport(files, ruleCount, findings, llmRes)
		if maxSeverity(findings) >= severityRankError {
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "repo-kb: unknown subcommand '%s'. Try 'help'.\n", sub)
		return 2
	}
}
