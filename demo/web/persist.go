package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// hubStore persists one session's activity log as JSON Lines. A store is safe
// for concurrent use, so several clients folding into the same session can
// append without corrupting the file.
type hubStore struct {
	mu   sync.Mutex
	path string
}

// openHubStore returns a store rooted at dir, creating it if needed. Session
// ids are URL-safe base32, so they double as file names.
func openHubStore(dir, id string) (*hubStore, error) {
	// #nosec G703 -- dir is the fixed session root and id is URL-safe base32
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &hubStore{path: filepath.Join(dir, id+".jsonl")}, nil
}

// load reads every record from the log. A missing file is simply an empty
// log, so a fresh session and a lost one replay identically.
func (st *hubStore) load() ([]opRecord, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	// #nosec G703 -- st.path is dir/id.jsonl where id is URL-safe base32
	file, err := os.Open(st.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var records []opRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var rec opRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("persist: %s: line %d: %w", st.path, len(records)+1, err)
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

// append writes one record to the log as a JSON line.
func (st *hubStore) append(rec opRecord) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	file, err := os.OpenFile(st.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}
