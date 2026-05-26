package adversarial

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func safeRunID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return !strings.Contains(id, "..")
}

func prepareOutputParent(rootAbs, dst string) error {
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return err
	}
	parent := filepath.Dir(dst)
	if err := existingPathInsideRoot(rootAbs, rootReal, parent); err != nil {
		return err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	parentReal, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	if !insideDir(rootReal, parentReal) {
		return fmt.Errorf("output path %q resolves outside repo root %q", dst, rootReal)
	}
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path %q is a symlink", dst)
	}
	return nil
}

func existingPathInsideRoot(rootAbs, rootReal, target string) error {
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output path %q must stay under repo root %q", target, rootAbs)
	}
	cur := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(cur)
		if err != nil {
			return err
		}
		if !insideDir(rootReal, real) {
			return fmt.Errorf("output path component %q resolves outside repo root %q", cur, rootReal)
		}
	}
	return nil
}

func insideDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
