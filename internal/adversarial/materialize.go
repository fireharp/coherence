package adversarial

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fireharp/coherence/internal/git"
	"github.com/fireharp/coherence/internal/glob"
	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/snapshot"
)

func materializeRepo(repo corpusRepo) (string, error) {
	dir, err := os.MkdirTemp("", "coherence-adversarial-")
	if err != nil {
		return "", err
	}
	cleanup := func(e error) (string, error) {
		os.RemoveAll(dir)
		return "", e
	}

	if len(repo.Files) > 0 {
		if err := writeFiles(dir, repo.Files); err != nil {
			return cleanup(err)
		}
	} else {
		if err := copyTracked(repo, dir); err != nil {
			return cleanup(err)
		}
	}

	if _, err := runGit(dir, "init", "-q"); err != nil {
		return cleanup(fmt.Errorf("git init: %w", err))
	}
	if err := ignoreRuntimeState(dir); err != nil {
		return cleanup(err)
	}
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return cleanup(fmt.Errorf("git add baseline: %w", err))
	}
	if _, err := runGit(dir,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		return cleanup(fmt.Errorf("git commit baseline: %w", err))
	}
	snap, err := snapshot.Compute(dir)
	if err != nil {
		return cleanup(fmt.Errorf("baseline snapshot: %w", err))
	}
	if err := snapshot.Write(dir, snap); err != nil {
		return cleanup(fmt.Errorf("baseline snapshot write: %w", err))
	}
	g, err := graph.Build(dir)
	if err != nil {
		return cleanup(fmt.Errorf("baseline graph: %w", err))
	}
	if err := graph.Write(dir, g); err != nil {
		return cleanup(fmt.Errorf("baseline graph write: %w", err))
	}
	return dir, nil
}

func writeFiles(root string, files map[string]string) error {
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func ignoreRuntimeState(root string) error {
	exclude := filepath.Join(root, ".git", "info", "exclude")
	data, err := os.ReadFile(exclude)
	if err != nil {
		return err
	}
	if hasIgnoreLine(data, ".coherence") {
		return nil
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, []byte(".coherence/\n")...)
	return os.WriteFile(exclude, data, 0o644)
}

func hasIgnoreLine(data []byte, want string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == want || line == want+"/" {
			return true
		}
	}
	return false
}

func copyTracked(repo corpusRepo, dst string) error {
	tracked := git.LsFiles(repo.Path)
	if len(tracked) == 0 {
		return fmt.Errorf("repo %s has no tracked files or is not a git repo: %s", repo.ID, repo.Path)
	}
	copied := 0
	for _, rel := range tracked {
		if !included(rel, repo.Include, repo.Exclude) {
			continue
		}
		srcAbs := filepath.Join(repo.Path, filepath.FromSlash(rel))
		dstAbs := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
			return err
		}
		if err := copyFile(srcAbs, dstAbs, rel); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		copied++
	}
	if copied == 0 {
		return fmt.Errorf("repo %s include/exclude filters selected no tracked files", repo.ID)
	}
	return nil
}

func included(path string, include, exclude []string) bool {
	ok := len(include) == 0
	for _, p := range include {
		if glob.Match(p, path) {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	for _, p := range exclude {
		if glob.Match(p, path) {
			return false
		}
	}
	return true
}

func copyFile(src, dst, rel string) error {
	lstat, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if lstat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if !safeSymlinkTarget(rel, target) {
			return fmt.Errorf("unsafe symlink target %q", target)
		}
		return os.Symlink(target, dst)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func safeSymlinkTarget(linkRel, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || filepath.IsAbs(target) {
		return false
	}
	linkDir := filepath.Dir(filepath.FromSlash(linkRel))
	clean := filepath.Clean(filepath.Join(linkDir, filepath.FromSlash(target)))
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
