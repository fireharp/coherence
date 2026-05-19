package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/format"
	"go/parser"
	"go/token"
)

// goSemantic returns a comment-and-whitespace-insensitive hash of a Go
// source file. Parses the file *without* comments and re-prints via
// go/format to produce a canonical form, then SHA-256s the result. If
// the file isn't valid Go, returns ok=false and the caller falls back
// to the content hash. This lets meters like stale_tests ignore
// comment-only edits to source files.
func goSemantic(body []byte) (string, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", body, parser.SkipObjectResolution)
	if err != nil {
		return "", false
	}
	// Reset comments so format.Node emits only declarations + bodies.
	file.Comments = nil
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return "", false
	}
	h := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(h[:]), true
}
