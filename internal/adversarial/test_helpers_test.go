package adversarial

import (
	"io"
	"math/rand"
	"net/http"
	"strings"
)

type fakeHTTPClient struct {
	body string
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

func randForTest() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

func firstLLMSpec() (Spec, bool) {
	for _, spec := range BuiltinSpecs() {
		if spec.RequiresLLM {
			return spec, true
		}
	}
	return Spec{}, false
}

func findResult(results []Result, mutationID string) *Result {
	for i := range results {
		if results[i].MutationID == mutationID {
			return &results[i]
		}
	}
	return nil
}

func resultSignatures(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, strings.Join([]string{
			r.RepoID,
			r.MutationID,
			r.TargetNode.ID,
			strings.Join(r.ExpectedMeters, ","),
			strings.Join(r.ActualMeters, ","),
			r.Classification,
			strings.Join(r.FalseNegatives, ","),
			strings.Join(r.FalsePositives, ","),
			r.SkipReason,
			r.Error,
		}, "|"))
	}
	return out
}
