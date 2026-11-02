package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo describes one stored session transcript.
type SessionInfo struct {
	// ID is the session's timestamp-slug id (file name without .jsonl).
	ID string
	// Path is the full path of the transcript file.
	Path string
	// CreatedAt is the transcript file's modification time (best available
	// proxy for creation time).
	CreatedAt time.Time
	// LineCount is the number of JSONL lines in the transcript.
	LineCount int
}

// List scans the sessions directory and returns one SessionInfo per *.jsonl
// file, newest first.
func (s *Store) List() ([]SessionInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("session: read sessions dir: %w", err)
	}
	var out []SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("session: stat %s: %w", path, err)
		}
		n, err := countLines(path)
		if err != nil {
			return nil, fmt.Errorf("session: count lines in %s: %w", path, err)
		}
		out = append(out, SessionInfo{
			ID:        strings.TrimSuffix(e.Name(), ".jsonl"),
			Path:      path,
			CreatedAt: info.ModTime(),
			LineCount: n,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// countLines counts non-empty lines in the file at path.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n, nil
}
