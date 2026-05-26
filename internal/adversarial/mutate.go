package adversarial

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func applyMutation(root string, spec Spec, target Target) error {
	switch spec.Operation {
	case opReplaceText:
		p := editPath(spec, target)
		if p == "" {
			return fmt.Errorf("replace_text: no target path")
		}
		return replaceText(root, p, spec.Edit.Old, spec.Edit.New)
	case opAppendText:
		p := editPath(spec, target)
		if p == "" {
			return fmt.Errorf("append_text: no target path")
		}
		return appendText(root, p, spec.Edit.Text)
	case opRemoveFile:
		p := editPath(spec, target)
		if p == "" {
			return fmt.Errorf("remove_file: no target path")
		}
		return removeFile(root, p)
	case opRenameFile:
		p := editPath(spec, target)
		if p == "" || spec.Edit.NewPath == "" {
			return fmt.Errorf("rename_file: path and new_path required")
		}
		return renameFile(root, p, renderTemplate(spec.Edit.NewPath, target))
	case opAddFile:
		if spec.Edit.Path == "" {
			return fmt.Errorf("add_file: path required")
		}
		return addFile(root, renderTemplate(spec.Edit.Path, target), renderTemplate(spec.Edit.Content, target))
	case opRemoveLineContaining:
		p := editPath(spec, target)
		if p == "" {
			return fmt.Errorf("remove_line_containing: no target path")
		}
		return removeLineContaining(root, p, spec.Edit.LineContains)
	case opBackdateHead:
		days := spec.Edit.AgeDays
		if days <= 0 {
			days = 120
		}
		return backdateHead(root, days)
	default:
		return fmt.Errorf("unsupported operation %q", spec.Operation)
	}
}

func editPath(spec Spec, target Target) string {
	if spec.Edit.Path != "" {
		return renderTemplate(spec.Edit.Path, target)
	}
	return target.Path
}

func renderTemplate(s string, target Target) string {
	out := strings.ReplaceAll(s, "${target_id}", target.ID)
	out = strings.ReplaceAll(out, "${target_path}", target.Path)
	out = strings.ReplaceAll(out, "${target_label}", target.Label)
	out = strings.ReplaceAll(out, "${target_dir}", path.Dir(target.Path))
	base := path.Base(target.Path)
	out = strings.ReplaceAll(out, "${target_base}", strings.TrimSuffix(base, path.Ext(base)))
	out = strings.ReplaceAll(out, "${target_ext}", path.Ext(base))
	return out
}

func replaceText(root, rel, old, new string) error {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	body := string(data)
	if !strings.Contains(body, old) {
		return fmt.Errorf("%s does not contain replacement text", rel)
	}
	body = strings.Replace(body, old, new, 1)
	return os.WriteFile(abs, []byte(body), 0o644)
}

func appendText(root, rel, text string) error {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func removeFile(root, rel string) error {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return err
	}
	err = os.Remove(abs)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", rel)
	}
	return err
}

func renameFile(root, oldRel, newRel string) error {
	oldAbs, err := safeJoin(root, oldRel)
	if err != nil {
		return err
	}
	newAbs, err := safeJoin(root, newRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}

func addFile(root, rel, content string) error {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

func removeLineContaining(root, rel, needle string) error {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if strings.Contains(line, needle) {
			removed = true
			continue
		}
		out = append(out, line)
	}
	if !removed {
		return fmt.Errorf("%s has no line containing %q", rel, needle)
	}
	return os.WriteFile(abs, []byte(strings.Join(out, "\n")), 0o644)
}

func safeJoin(root, rel string) (string, error) {
	if !safeRelativePath(rel) {
		return "", fmt.Errorf("unsafe in-repo path %q", rel)
	}
	return filepath.Join(root, filepath.Clean(filepath.FromSlash(rel))), nil
}

func backdateHead(root string, days int) error {
	when := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	cmd := exec.Command("git", "-C", root,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "--amend", "--no-edit", "--date", when)
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+when, "GIT_AUTHOR_DATE="+when)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git backdate head: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
