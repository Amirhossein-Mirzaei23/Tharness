package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sabhiram/go-gitignore"
)

// gitignoreMatcher wraps the go-gitignore CompileIgnoreFileAndLines result so
// the walk helper stays dependency-agnostic.
type gitignoreMatcher struct {
	ig ignore.IgnoreParser
}

// newGitignoreMatcher compiles gitignore rules from raw file content. Invalid
// lines are ignored by the underlying library.
func newGitignoreMatcher(data []byte) *gitignoreMatcher {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	return &gitignoreMatcher{ig: ignore.CompileIgnoreLines(lines...)}
}

// MatchesPath reports whether a slash-separated relative path is ignored.
func (g *gitignoreMatcher) MatchesPath(rel string) bool {
	return g.ig.MatchesPath(rel)
}

// readGitignoreFile returns the contents of <root>/.gitignore, or nil when the
// file does not exist.
func readGitignoreFile(root string) ([]byte, error) {
	p := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
