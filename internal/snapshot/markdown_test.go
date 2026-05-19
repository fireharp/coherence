package snapshot

import (
	"testing"
)

func TestMarkdownSemanticIgnoresProseTypo(t *testing.T) {
	a := markdownSemantic([]byte(`---
title: Auth
status: draft
---

# Authentication

This is a short paragraph that explains the auth flow.

See [the spec](docs/specs/auth.md) for details.
`))
	b := markdownSemantic([]byte(`---
title: Auth
status: draft
---

# Authentication

This is a short paragrahp that explians the auth flow.

See [the spec](docs/specs/auth.md) for details.
`))
	if a != b {
		t.Errorf("typo-only edit changed semantic hash:\n  a=%s\n  b=%s", a, b)
	}
}

func TestMarkdownSemanticChangesOnHeadingRename(t *testing.T) {
	a := markdownSemantic([]byte("# Old Heading\n\nprose.\n"))
	b := markdownSemantic([]byte("# New Heading\n\nprose.\n"))
	if a == b {
		t.Errorf("heading rename should change semantic hash, both = %s", a)
	}
}

func TestMarkdownSemanticChangesOnLinkTargetSwap(t *testing.T) {
	a := markdownSemantic([]byte("See [doc](old/path.md).\n"))
	b := markdownSemantic([]byte("See [doc](new/path.md).\n"))
	if a == b {
		t.Errorf("link target swap should change semantic hash, both = %s", a)
	}
}

func TestMarkdownSemanticChangesOnFrontmatterValue(t *testing.T) {
	a := markdownSemantic([]byte("---\ntitle: A\nstatus: draft\n---\n\nbody.\n"))
	b := markdownSemantic([]byte("---\ntitle: A\nstatus: active\n---\n\nbody.\n"))
	if a == b {
		t.Errorf("frontmatter value change should change semantic hash, both = %s", a)
	}
}

func TestMarkdownSemanticChangesOnFenceLang(t *testing.T) {
	a := markdownSemantic([]byte("# t\n\n```bash\necho hi\n```\n"))
	b := markdownSemantic([]byte("# t\n\n```sh\necho hi\n```\n"))
	if a == b {
		t.Errorf("fence lang change should change semantic hash, both = %s", a)
	}
}

func TestSplitFrontmatterNoFrontmatter(t *testing.T) {
	fm, body := splitFrontmatter([]byte("# Heading\n\nbody."))
	if fm != nil {
		t.Errorf("expected no frontmatter, got %q", string(fm))
	}
	if string(body) != "# Heading\n\nbody." {
		t.Errorf("body altered: %q", string(body))
	}
}

func TestSplitFrontmatterPresent(t *testing.T) {
	fm, body := splitFrontmatter([]byte("---\ntitle: A\n---\n# Heading\n"))
	if string(fm) != "title: A" {
		t.Errorf("frontmatter = %q", string(fm))
	}
	if string(body) != "# Heading\n" {
		t.Errorf("body = %q", string(body))
	}
}
