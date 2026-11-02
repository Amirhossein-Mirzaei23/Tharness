package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// grepMaxMatches caps the number of result lines returned by the grep tool.
const grepMaxMatches = 200

// GrepTool searches file contents in the workspace with a regular expression,
// returning "path:line: text" per match. The walk skips .git and honors a
// root-level .gitignore (shared with the glob tool). Binary-ish files (NUL
// byte in the first 512 bytes) are skipped.
type GrepTool struct {
	root string
	opts GrepOptions
}

// GrepOptions configures the grep tool.
type GrepOptions struct {
	// MaxMatches caps the number of returned match lines (default 200).
	MaxMatches int
}

// NewGrepTool returns a GrepTool rooted at the absolute workspace dir.
func NewGrepTool(root string, opts GrepOptions) *GrepTool {
	if opts.MaxMatches <= 0 {
		opts.MaxMatches = grepMaxMatches
	}
	return &GrepTool{root: root, opts: opts}
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents in the workspace with a regular expression. Returns \"path:line: text\" per match, capped at 200 matches. Respects .gitignore, skips .git and binary files."
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern":     map[string]any{"type": "string", "description": "Go regular expression to search for"},
			"glob":        map[string]any{"type": "string", "description": "Optional glob filter on file paths, e.g. \"*.md\""},
			"ignore_case": map[string]any{"type": "boolean", "description": "Case-insensitive search (default false)"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Pattern    string `json:"pattern"`
		Glob       string `json:"glob"`
		IgnoreCase bool   `json:"ignore_case"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("grep: parse args: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("grep: pattern is required")
	}
	expr := args.Pattern
	if args.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("grep: invalid pattern: %w", err)
	}
	var globRE *regexp.Regexp
	if args.Glob != "" {
		globRE, err = compileGlobFilter(args.Glob)
		if err != nil {
			return "", fmt.Errorf("grep: invalid glob: %w", err)
		}
	}

	ignore, err := loadRootGitignore(t.root)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}

	var out []string
	capped := false
	werr := filepath.WalkDir(t.root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := relFromRoot(t.root, p)
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
		if globRE != nil && !globRE.MatchString(rel) {
			return nil
		}
		lines, lerr := matchingLines(p, rel, re, t.opts.MaxMatches-len(out))
		if lerr != nil {
			return lerr
		}
		out = append(out, lines...)
		if len(out) >= t.opts.MaxMatches {
			out = out[:t.opts.MaxMatches]
			capped = true
			return filepath.SkipAll
		}
		return nil
	})
	if werr != nil {
		return "", fmt.Errorf("grep: %w", werr)
	}
	if len(out) == 0 {
		return "no matches", nil
	}
	result := strings.Join(out, "\n")
	if capped {
		result += fmt.Sprintf("\n[truncated: showing first %d matches]", t.opts.MaxMatches)
	}
	return result, nil
}

// compileGlobFilter converts a user glob like "*.md" into a regexp matching
// relative slash-separated paths. The glob applies to a suffix of the path:
// "*.md" matches docs/notes.md; "**" as a whole segment spans any depth;
// "*" and "?" never cross a "/" boundary.
func compileGlobFilter(glob string) (*regexp.Regexp, error) {
	segments := strings.Split(filepath.ToSlash(filepath.Clean(glob)), "/")
	var b strings.Builder
	b.WriteString("(^|/)")
	for i, seg := range segments {
		if seg == "**" {
			b.WriteString(".*")
			continue
		}
		if i > 0 {
			b.WriteString("/")
		}
		for _, r := range seg {
			switch r {
			case '*':
				b.WriteString("[^/]*")
			case '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// matchingLines scans one file and returns "rel:lineNo: text" strings for
// lines matching re, up to max of them. Files with a NUL byte in the first
// 512 bytes are treated as binary and skipped.
func matchingLines(abs, rel string, re *regexp.Regexp, max int) ([]string, error) {
	if max <= 0 {
		return nil, nil
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // raced away; ignore
		}
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only closer

	// Binary sniff: NUL in first 512 bytes.
	head := make([]byte, 512)
	n, _ := f.Read(head)
	if n > 0 && strings.ContainsRune(string(head[:n]), 0) {
		return nil, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		if re.Match(sc.Bytes()) {
			out = append(out, fmt.Sprintf("%s:%d: %s", rel, lineNo, sc.Text()))
			if len(out) >= max {
				break
			}
		}
	}
	return out, sc.Err()
}
