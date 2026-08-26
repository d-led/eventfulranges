package collab

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// sessionStore persists one session's event log as JSON Lines under a data
// directory. A store is safe for concurrent use, so several clients folding
// into the same session can append without corrupting the file.
type sessionStore struct {
	mu   sync.Mutex
	path string
}

// openSessionStore returns a store rooted at dir, creating it if needed. The
// session id is already URL-safe base32, so it doubles as a file name.
func openSessionStore(dir, id string) (*sessionStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &sessionStore{path: filepath.Join(dir, id+".jsonl")}, nil
}

// load reads every entry from the log. A missing file is simply an empty log,
// so a fresh session and a lost one replay identically.
func (st *sessionStore) load() ([]Entry, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	file, err := os.Open(st.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("collab: %s: line %d: %w", st.path, len(entries)+1, err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// append writes the entries to the log as JSON Lines.
func (st *sessionStore) append(entries []Entry) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	file, err := os.OpenFile(st.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}
