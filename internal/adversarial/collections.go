package adversarial

import "sort"

func stringSet(in []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range in {
		if s != "" {
			out[s] = true
		}
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func appendUnique(in []string, vals ...string) []string {
	seen := map[string]bool{}
	for _, v := range in {
		seen[v] = true
	}
	for _, v := range vals {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		in = append(in, v)
	}
	return in
}
