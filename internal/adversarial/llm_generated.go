package adversarial

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/fireharp/coherence/internal/graph"
)

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
