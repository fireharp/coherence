package adversarial

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fireharp/coherence/internal/graph"
)

const defaultGroqEndpoint = "https://api.groq.com/openai/v1/chat/completions"

type llmChatRequest struct {
	Model       string       `json:"model"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
	Messages    []llmMessage `json:"messages"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmChatResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
}

// GenerateLLMSpecs asks Groq for additional mutation specs using only graph
// summaries and the DSL schema. It returns no specs when disabled by missing
// credentials, keeping deterministic bench runs stable.
func GenerateLLMSpecs(opts Options, repos []corpusRepo, existing []Spec) ([]Spec, error) {
	if os.Getenv("GROQ_API_KEY") == "" {
		return nil, nil
	}
	if len(repos) == 0 {
		return nil, nil
	}
	dir, err := materializeRepo(repos[0])
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	g, err := graph.Load(dir)
	if err != nil {
		return nil, err
	}
	payload := llmChatRequest{
		Model:       groqModel(),
		MaxTokens:   1200,
		Temperature: 0,
		Messages: []llmMessage{
			{Role: "system", Content: "You generate adversarial benchmark mutation specs for coherence. Return only JSON."},
			{Role: "user", Content: llmSpecPrompt(g, existing)},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := opts.GroqEndpoint
	if endpoint == "" {
		endpoint = defaultGroqEndpoint
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))
	client := opts.GroqHTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("groq %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed llmChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, nil
	}
	specs, err := parseLLMSpecs(parsed.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	if err := validateSpecs(specs); err != nil {
		return nil, err
	}
	specs = applicableGeneratedSpecs(repos[0], specs, existing)
	if err := writeGeneratedSpecs(opts.RootDir, specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func groqModel() string {
	if m := os.Getenv("COHERENCE_GROQ_MODEL"); m != "" {
		return m
	}
	return "llama-3.3-70b-versatile"
}

func llmSpecPrompt(g graph.Graph, existing []Spec) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Return JSON shaped exactly as: {\"version\":1,\"mutations\":[...]}.")
	fmt.Fprintln(&b, "Each mutation must include id, operation, target_kinds, expected_meters, selector, edit.")
	fmt.Fprintln(&b, "Use optional skip_conditions.require_env, require_files, or require_optional_engines for explicit preconditions.")
	fmt.Fprintf(&b, "Allowed operations: %s.\n", strings.Join(operationNames(), ", "))
	fmt.Fprintln(&b, "Do not include repository file contents. Use only this graph summary.")
	fmt.Fprintf(&b, "Existing mutation ids to avoid: %s.\n\n", strings.Join(specIDs(existing), ", "))
	fmt.Fprintln(&b, "Graph nodes:")
	for _, line := range graphNodeSummary(g, 80) {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintln(&b, "Graph edges:")
	for _, line := range graphEdgeSummary(g, 120) {
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

func operationNames() []string {
	out := make([]string, 0, len(validOperations))
	for op := range validOperations {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

func graphNodeSummary(g graph.Graph, max int) []string {
	nodes := append([]graph.Node(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		return nodes[i].ID < nodes[j].ID
	})
	out := []string{}
	for i, n := range nodes {
		if i >= max {
			break
		}
		out = append(out, fmt.Sprintf("- kind=%s id=%s path=%s", n.Kind, n.ID, n.Path))
	}
	return out
}

func graphEdgeSummary(g graph.Graph, max int) []string {
	edges := append([]graph.Edge(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	out := []string{}
	for i, e := range edges {
		if i >= max {
			break
		}
		out = append(out, fmt.Sprintf("- kind=%s from=%s to=%s", e.Kind, e.From, e.To))
	}
	return out
}

func parseLLMSpecs(content string) ([]Spec, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var tf TaxonomyFile
	if err := json.Unmarshal([]byte(content), &tf); err != nil {
		return nil, err
	}
	if tf.Version == 0 {
		tf.Version = 1
	}
	if tf.Version != 1 {
		return nil, fmt.Errorf("generated taxonomy unsupported version %d", tf.Version)
	}
	return tf.Mutation, nil
}

func applicableGeneratedSpecs(repo corpusRepo, generated, existing []Spec) []Spec {
	seen := map[string]bool{}
	for _, s := range existing {
		seen[s.ID] = true
	}
	out := []Spec{}
	for _, s := range generated {
		if seen[s.ID] {
			continue
		}
		if _, ok := envSkipReason(s); ok {
			continue
		}
		if !dryRunApplicable(repo, s) {
			continue
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out
}

func dryRunApplicable(repo corpusRepo, spec Spec) bool {
	dir, err := materializeRepo(repo)
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	if _, ok := optionalEngineSkipReason(dir, spec); ok {
		return false
	}
	g, err := graph.Load(dir)
	if err != nil {
		return false
	}
	target := Target{}
	if spec.Operation != opBackdateHead {
		var ok bool
		target, ok = selectTarget(g, spec, randForApplicability(spec.ID))
		if !ok {
			return false
		}
	}
	if _, ok := fileSkipReason(dir, spec); ok {
		return false
	}
	return applyMutation(dir, spec, target) == nil
}

func randForApplicability(id string) *rand.Rand {
	sum := sha256.Sum256([]byte(id))
	seed := int64(binary.BigEndian.Uint64(sum[:8]))
	return rand.New(rand.NewSource(seed))
}

func writeGeneratedSpecs(rootDir string, specs []Spec) error {
	if len(specs) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(TaxonomyFile{Version: 1, Mutation: specs}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return err
	}
	stem := "llm-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	for i := 0; i < 1000; i++ {
		name := stem
		if i > 0 {
			name += fmt.Sprintf("-%03d", i)
		}
		dst := filepath.Join(rootAbs, ".coherence", "adversarial", "specs", name+".json")
		if err := prepareOutputParent(rootAbs, dst); err != nil {
			return err
		}
		if err := writeFileExclusive(dst, data, 0o644); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("could not allocate generated spec filename under %s", filepath.Join(rootAbs, ".coherence", "adversarial", "specs"))
}

func writeFileExclusive(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
