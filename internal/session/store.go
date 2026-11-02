// Package session implements the append-only JSONL session store used for
// conversation transcripts and resume.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages session transcripts under <dir>/sessions.
type Store struct {
	dir string // <dir>/sessions
	mu  sync.Mutex
}

// Open creates the sessions directory (if needed) and returns a Store rooted
// at it.
func Open(dir string) (*Store, error) {
	sdir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		return nil, fmt.Errorf("session: create sessions dir: %w", err)
	}
	return &Store{dir: sdir}, nil
}

// New starts a fresh session: it returns a unique timestamp-slug id (e.g.
// 20261102-150405-ab12cd) and the path of its transcript file.
func (s *Store) New() (id string, path string, err error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("session: new id: %w", err)
	}
	id = time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
	return id, s.Path(id), nil
}

// Path returns the transcript file path for the given session id.
func (s *Store) Path(id string) string {
	return filepath.Join(s.dir, id+".jsonl")
}
