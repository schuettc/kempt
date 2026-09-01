package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State holds the user's machine selection.
type State struct {
	RepoDir string `json:"repo_dir"`
	RepoURL string `json:"repo_url,omitempty"`
	// RepoKind is "git" (default/empty) or "tarball"; it tells `update` whether
	// to git-pull the repo or re-fetch and re-extract the tarball at RepoURL.
	RepoKind       string   `json:"repo_kind,omitempty"`
	Profile        string   `json:"profile,omitempty"`
	Packages       []string `json:"packages"`
	AutoApplyFiles bool     `json:"auto_apply_files"`
}

// Status holds the cached result of the last refresh check.
type Status struct {
	CheckedAt       time.Time `json:"checked_at"`
	Behind          int       `json:"behind"`
	FileChanges     int       `json:"file_changes"`
	SoftwareChanges int       `json:"software_changes"`
	Blocked         int       `json:"blocked"`
}

// Store is the persistence layer; state.json and status.json live in Dir.
type Store struct{ Dir string }

// DefaultStore resolves XDG_DATA_HOME (or ~/.local/share) joined with "kempt".
func DefaultStore() (*Store, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return &Store{Dir: filepath.Join(base, "kempt")}, nil
}

// Load reads state.json. Missing file returns (&State{}, false, nil).
func (s *Store) Load() (*State, bool, error) {
	data, existed, err := readFile(filepath.Join(s.Dir, "state.json"))
	if err != nil || !existed {
		return &State{}, false, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, false, err
	}
	return &st, true, nil
}

// Save writes state.json atomically (marshal → .tmp → rename), creating Dir as needed.
func (s *Store) Save(st *State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return atomicWrite(s.Dir, "state.json", data)
}

// LoadStatus reads status.json. Missing file returns (&Status{}, false, nil).
func (s *Store) LoadStatus() (*Status, bool, error) {
	data, existed, err := readFile(filepath.Join(s.Dir, "status.json"))
	if err != nil || !existed {
		return &Status{}, false, err
	}
	var st Status
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, false, err
	}
	return &st, true, nil
}

// SaveStatus writes status.json atomically, creating Dir as needed.
func (s *Store) SaveStatus(st *Status) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return atomicWrite(s.Dir, "status.json", data)
}

// readFile reads a file; returns (nil, false, nil) if it doesn't exist.
func readFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// atomicWrite marshals data to dir/<name>.tmp then renames to dir/<name>.
func atomicWrite(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, name+".tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}
