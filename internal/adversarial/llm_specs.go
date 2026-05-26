package adversarial

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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
