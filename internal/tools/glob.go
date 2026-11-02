package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// globMaxMatches caps the number of results returned by the glob tool.
const globMaxMatches = 500

// GlobTool lists workspace files matching a pattern relative to the workspace
// root. Patterns use "/"-separated segments where "**" matches zero or more
// path segments (a simple hand-rolled doublestar matcher on top of
// path.Match; see matchGlobSegments) and every other segment uses the usual
// filepath.Match syntax (?, *, [...]).
//
// The walk skips the .git directory and honors a root-level .gitignore file
// when present (subdirectory .gitignore files are not evaluated).
type GlobTool struct {
	root string
	opts GlobOptions
}

// GlobOptions configures the glob tool.
type GlobOptions struct {
	// MaxMatches caps the number of returned matches (default 500).
	MaxMatches int
}

// NewGlobTool returns a GlobTool rooted at the absolute workspace dir.
func NewGlobTool(root string, opts GlobOptions) *GlobTool {
	if opts.MaxMatches <= 0 {
		opts.MaxMatches = globMaxMatches
	}
	return &GlobTool{root: root, opts: opts}
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return "List files in the workspace matching a glob pattern relative to the workspace root, e.g. \"**/*.go\". Respects .gitignore and skips .git. Returns relative paths."
}

func (t *GlobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Glob pattern relative to the workspace root; ** matches any number of path segments, * and ? match within a single segment"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("glob: parse args: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("glob: pattern is required")
	}

	ignore, err := loadRootGitignore(t.root)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	matches, capped, err := walkMatches(t.root, ignore, func(rel string) bool {
		return matchGlobPattern(args.Pattern, rel)
	}, t.opts.MaxMatches)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	if len(matches) == 0 {
		return "no matches", nil
	}
	out := strings.Join(matches, "\n")
	if capped {
		out += fmt.Sprintf("\n[truncated: showing first %d matches]", t.opts.MaxMatches)
	}
	return out, nil
}

// matchGlobPattern reports whether rel (slash-separated, cleaned, relative)
// matches pattern. "**" as a whole segment matches zero or more segments;
// other segments are matched with path.Match.
func matchGlobPattern(pattern, rel string) bool {
	patSegs := strings.Split(filepath.ToSlash(filepath.Clean(pattern)), "/")
	relSegs := strings.Split(rel, "/")
	return matchGlobSegments(patSegs, relSegs)
}

// matchGlobSegments is a simple recursive doublestar matcher: "**" consumes
// zero or more segments, everything else must match one segment exactly.
func matchGlobSegments(pat, seg []string) bool {
	if len(pat) == 0 {
		return len(seg) == 0
	}
	if pat[0] == "**" {
		// "**" matches zero segments or consumes one and stays.
		return matchGlobSegments(pat[1:], seg) ||
			(len(seg) > 0 && matchGlobSegments(pat, seg[1:]))
	}
	if len(seg) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], seg[0])
	if err != nil || !ok {
		return false
	}
	return matchGlobSegments(pat[1:], seg[1:])
}

// loadRootGitignore parses <root>/.gitignore if present, returning nil when
// absent.
func loadRootGitignore(root string) (*gitignoreMatcher, error) {
	data, err := readGitignoreFile(root)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return newGitignoreMatcher(data), nil
}

// walkMatches walks root (skipping .git and gitignored paths), invoking want
// on each cleaned relative slash-separated path, and returns up to max
// matches in sorted order plus whether the cap was hit.
func walkMatches(root string, ignore *gitignoreMatcher, want func(rel string) bool, max int) (matches []string, capped bool, err error) {
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := relFromRoot(root, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if ignore != nil && rel != "." && ignore.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore != nil && ignore.MatchesPath(rel) {
			return nil
		}
		if !want(rel) {
			return nil
		}
		if len(matches) < max {
			matches = append(matches, rel)
		}
		if len(matches) == max {
			capped = true
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(matches)
	return matches, capped, err
}

// relFromRoot returns the slash-separated path of p relative to root.
func relFromRoot(root, p string) (string, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
